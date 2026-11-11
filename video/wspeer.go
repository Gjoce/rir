package video

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// wsWriteTimeout caps how long one outbound message may block on the
// underlying TCP write before we give up and close the peer. Set higher than
// any realistic latency for a small JSON envelope; if it's exceeded, the
// client is genuinely stuck and should be torn down.
const wsWriteTimeout = 10 * time.Second

// wsSendBuffer bounds the number of in-flight outbound messages per peer. SDP
// + ICE traffic is bursty for a few hundred ms then mostly silent, so a small
// buffer is enough; if it fills, the peer is too slow to keep up and we drop
// the message rather than letting the broker block.
const wsSendBuffer = 32

// ErrPeerClosed is returned by WSPeer.Send when the peer has been closed.
var ErrPeerClosed = errors.New("video: peer closed")

// ErrSendBufferFull is returned by WSPeer.Send when the outbound queue is
// saturated. Callers (the broker) should treat this as a transient drop, not
// a fatal error — WebRTC clients will renegotiate.
var ErrSendBufferFull = errors.New("video: send buffer full")

// WSPeer implements Peer over a coder/websocket connection. Outbound writes
// are serialized through a single goroutine fed by a buffered channel so the
// broker never blocks on a slow client.
type WSPeer struct {
	conn *websocket.Conn
	out  chan Message

	closed atomic.Bool
	once   sync.Once
	done   chan struct{}
}

// NewWSPeer wraps conn in a WSPeer and starts its write loop. The caller is
// responsible for invoking Close exactly once (typically via defer) when the
// peer is no longer wanted, and for running the read loop separately.
func NewWSPeer(conn *websocket.Conn) *WSPeer {
	p := &WSPeer{
		conn: conn,
		out:  make(chan Message, wsSendBuffer),
		done: make(chan struct{}),
	}
	go p.writeLoop()

	return p
}

// Send queues msg for delivery. Non-blocking: if the outbound buffer is full,
// the message is dropped and ErrSendBufferFull is returned.
func (p *WSPeer) Send(msg Message) error {
	if p.closed.Load() {
		return ErrPeerClosed
	}
	select {
	case p.out <- msg:
		return nil
	default:
		return ErrSendBufferFull
	}
}

// Done returns a channel that is closed when the peer is closed. Useful for a
// caller running a parallel read loop that needs to terminate when writes
// fail and the peer self-closes.
func (p *WSPeer) Done() <-chan struct{} {
	return p.done
}

// Close terminates the write loop and closes the underlying websocket. Safe
// to call multiple times; only the first call has effect.
func (p *WSPeer) Close() {
	p.once.Do(func() {
		p.closed.Store(true)
		close(p.done)
		_ = p.conn.Close(websocket.StatusNormalClosure, "")
	})
}

// writeLoop serializes outbound messages until Close is called or a write
// fails. A failed write closes the peer, which is the right call: if the
// client can't receive, the call is over.
func (p *WSPeer) writeLoop() {
	for {
		select {
		case <-p.done:
			return
		case msg := <-p.out:
			ctx, cancel := context.WithTimeout(context.Background(), wsWriteTimeout)
			err := writeJSON(ctx, p.conn, msg)
			cancel()
			if err != nil {
				p.Close()

				return
			}
		}
	}
}

// writeJSON marshals msg and writes it as a single text frame. Extracted so
// tests can confirm framing without standing up a full WebSocket.
func writeJSON(ctx context.Context, conn *websocket.Conn, msg Message) error {
	buf, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return conn.Write(ctx, websocket.MessageText, buf)
}
