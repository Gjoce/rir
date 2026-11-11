package video_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"cybermedico/video"

	"github.com/coder/websocket"
)

// wsTestServer accepts one WebSocket connection, hands it to acceptedFn, and
// keeps the connection open until acceptedFn returns. Returned URL uses ws://.
func wsTestServer(t *testing.T, acceptedFn func(conn *websocket.Conn)) (url string, cleanup func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{"*"},
		})
		if err != nil {
			t.Errorf("accept: %v", err)

			return
		}
		acceptedFn(conn)
	}))
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	return wsURL, srv.Close
}

func TestWSPeerSendDeliversMessage(t *testing.T) {
	var (
		mu      sync.Mutex
		peerRef *video.WSPeer
		peerSet = make(chan struct{})
	)

	url, cleanup := wsTestServer(t, func(conn *websocket.Conn) {
		mu.Lock()
		peerRef = video.NewWSPeer(conn)
		mu.Unlock()
		close(peerSet)

		// Keep the goroutine alive until the test signals teardown by
		// closing the peer. We block here.
		<-peerRef.Done()
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() {
		if err := client.CloseNow(); err != nil {
			t.Logf("doctorConn.CloseNow: %v", err)
		}
	}()

	<-peerSet
	mu.Lock()
	peer := peerRef
	mu.Unlock()

	if err := peer.Send(video.Message{Type: video.MsgOffer}); err != nil {
		t.Fatalf("send: %v", err)
	}

	_, data, err := client.Read(ctx)
	if err != nil {
		t.Fatalf("client read: %v", err)
	}

	var got video.Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Type != video.MsgOffer {
		t.Errorf("got.Type = %q, want %q", got.Type, video.MsgOffer)
	}

	peer.Close()
}

func TestWSPeerSendAfterCloseReturnsError(t *testing.T) {
	var peer *video.WSPeer
	ready := make(chan struct{})

	url, cleanup := wsTestServer(t, func(conn *websocket.Conn) {
		peer = video.NewWSPeer(conn)
		close(ready)
		<-peer.Done()
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() {
		if err := client.CloseNow(); err != nil {
			t.Logf("doctorConn.CloseNow: %v", err)
		}
	}()

	<-ready
	peer.Close()

	err = peer.Send(video.Message{Type: video.MsgOffer})
	if !errors.Is(err, video.ErrPeerClosed) {
		t.Errorf("err = %v, want ErrPeerClosed", err)
	}
}

func TestWSPeerCloseIsIdempotent(t *testing.T) {
	var peer *video.WSPeer
	ready := make(chan struct{})

	url, cleanup := wsTestServer(t, func(conn *websocket.Conn) {
		peer = video.NewWSPeer(conn)
		close(ready)
		<-peer.Done()
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() {
		if err := client.CloseNow(); err != nil {
			t.Logf("doctorConn.CloseNow: %v", err)
		}
	}()

	<-ready
	peer.Close()
	peer.Close() // must not panic / deadlock
	peer.Close()
}
