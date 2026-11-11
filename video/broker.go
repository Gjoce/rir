package video

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Role identifies which side of a video consultation a peer is on. The signaling
// layer is intentionally limited to these two roles — there is never a third
// participant in a consultation room.
type Role string

const (
	RoleDoctor  Role = "doctor"
	RolePatient Role = "patient"
)

// MessageType enumerates every signaling envelope the broker understands or
// emits. SDP and ICE payloads ride inside Message.Data as opaque JSON; the
// broker never inspects them.
type MessageType string

const (
	MsgOffer        MessageType = "offer"
	MsgAnswer       MessageType = "answer"
	MsgICECandidate MessageType = "ice-candidate"
	MsgBye          MessageType = "bye"

	MsgPeerJoined   MessageType = "peer-joined"
	MsgPeerLeft     MessageType = "peer-left"
	MsgPeerRejoined MessageType = "peer-rejoined"
	MsgPeerPresent  MessageType = "peer-present"
)

// Message is the wire envelope exchanged with each peer.
type Message struct {
	Type MessageType     `json:"type"`
	From Role            `json:"from,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Peer is the broker's view of one side of a call. The WebSocket handler in
// PR2 will implement this; broker tests implement it with an in-memory fake.
// Send must be safe to call from multiple goroutines.
type Peer interface {
	Send(msg Message) error
	Close()
}

// ErrInvalidRole is returned when a caller passes a Role value that isn't one
// of the two recognized roles.
var ErrInvalidRole = errors.New("video: invalid role")

// Room holds the up-to-two peers attached to a single consultation. Peers
// keyed by role; replacing a peer of the same role is how reconnects work.
type Room struct {
	consultationID uuid.UUID

	mu           sync.Mutex
	peers        map[Role]Peer
	lastActivity time.Time
}

// Broker is the per-process registry of active signaling rooms. One Broker
// instance backs all consultations on a given API node. Horizontally scaling
// the signaling layer will require swapping this for a Redis-backed
// implementation; the public surface is small precisely so that swap stays
// cheap.
type Broker struct {
	mu    sync.Mutex
	rooms map[uuid.UUID]*Room

	// idleTTL is the grace period after a room's last activity (peer join,
	// leave, or relay) before EvictIdle considers it removable. Must be at
	// least as long as the auto-reconnect window so a brief drop doesn't
	// destroy the room out from under a returning peer.
	idleTTL time.Duration

	// clock is injectable for tests.
	clock func() time.Time
}

// NewBroker returns a broker with sensible defaults: 5-minute idle TTL,
// time.Now as the clock.
func NewBroker() *Broker {
	return &Broker{
		rooms:   make(map[uuid.UUID]*Room),
		idleTTL: 5 * time.Minute,
		clock:   time.Now,
	}
}

// SetClock overrides the broker's clock. Test-only.
func (b *Broker) SetClock(now func() time.Time) {
	b.clock = now
}

// SetIdleTTL overrides the broker's idle eviction TTL.
func (b *Broker) SetIdleTTL(d time.Duration) {
	b.idleTTL = d
}

// validRole reports whether r is one of the supported roles.
func validRole(r Role) bool {
	return r == RoleDoctor || r == RolePatient
}

// otherRole returns the role on the opposite side of a 1:1 call.
func otherRole(r Role) Role {
	if r == RoleDoctor {
		return RolePatient
	}

	return RoleDoctor
}

// getOrCreateRoom returns the room for consultationID, creating it if absent.
// Sets lastActivity to now on creation so EvictIdle cannot snipe a freshly
// created room before its first peer attaches.
func (b *Broker) getOrCreateRoom(consultationID uuid.UUID) *Room {
	b.mu.Lock()
	defer b.mu.Unlock()

	if room, ok := b.rooms[consultationID]; ok {
		return room
	}

	room := &Room{
		consultationID: consultationID,
		peers:          make(map[Role]Peer),
		lastActivity:   b.clock(),
	}
	b.rooms[consultationID] = room

	return room
}

// getRoom returns the existing room for consultationID, or nil if none exists.
func (b *Broker) getRoom(consultationID uuid.UUID) *Room {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.rooms[consultationID]
}

// Join attaches peer to the room for the given role. If a peer of the same
// role is already attached, the existing one is closed and replaced — this is
// the reconnect path. The peer on the other side, if present, is notified
// with PeerJoined (first connect) or PeerRejoined (reconnect, signaling that
// it should restart ICE).
func (b *Broker) Join(consultationID uuid.UUID, role Role, peer Peer) error {
	if !validRole(role) {
		return ErrInvalidRole
	}

	room := b.getOrCreateRoom(consultationID)

	room.mu.Lock()
	defer room.mu.Unlock()

	prev, hadPrev := room.peers[role]
	room.peers[role] = peer
	room.lastActivity = b.clock()

	if hadPrev {
		// Close after replacing so a Leave call from the old peer's goroutine
		// sees the new peer in the map and noops.
		prev.Close()
	}

	if other, ok := room.peers[otherRole(role)]; ok {
		notice := MsgPeerJoined
		if hadPrev {
			notice = MsgPeerRejoined
		}
		// Send errors from the *other* peer don't affect this Join.
		_ = other.Send(Message{Type: notice, From: role})

		_ = peer.Send(Message{Type: MsgPeerPresent, From: otherRole(role)})
	}

	return nil
}

// Leave removes peer from the room for the given role, but only if the stored
// peer is exactly the one passed in. This prevents an asynchronous cleanup
// goroutine on a replaced (reconnected-over) peer from accidentally evicting
// the new peer. The other side is notified with PeerLeft.
func (b *Broker) Leave(consultationID uuid.UUID, role Role, peer Peer) {
	if !validRole(role) {
		return
	}

	room := b.getRoom(consultationID)
	if room == nil {
		return
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	stored, ok := room.peers[role]
	if !ok || stored != peer {
		return
	}

	delete(room.peers, role)
	room.lastActivity = b.clock()

	if other, ok := room.peers[otherRole(role)]; ok {
		_ = other.Send(Message{Type: MsgPeerLeft, From: role})
	}
}

// Relay forwards msg from the peer holding role to the peer on the other
// side. If the other side is absent (not connected yet, briefly disconnected)
// the message is dropped — ICE candidates tolerate loss, and SDP renegotiation
// is handled by the client's perfect-negotiation logic when peers reattach.
//
// The broker overwrites msg.From with role so a malicious or buggy client
// cannot spoof the origin.
func (b *Broker) Relay(consultationID uuid.UUID, role Role, msg Message) {
	if !validRole(role) {
		return
	}

	room := b.getRoom(consultationID)
	if room == nil {
		return
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	room.lastActivity = b.clock()
	msg.From = role
	if other, ok := room.peers[otherRole(role)]; ok {
		_ = other.Send(msg)
	}
}

// EvictIdle removes rooms that have no peers and have been quiet for longer
// than idleTTL. Returns the number of rooms evicted. Intended to be called
// from a periodic goroutine in the broker's owner.
func (b *Broker) EvictIdle() int {
	cutoff := b.clock().Add(-b.idleTTL)

	b.mu.Lock()
	defer b.mu.Unlock()

	evicted := 0
	for cid, room := range b.rooms {
		room.mu.Lock()
		empty := len(room.peers) == 0
		idle := room.lastActivity.Before(cutoff)
		room.mu.Unlock()

		if empty && idle {
			delete(b.rooms, cid)
			evicted++
		}
	}

	return evicted
}

// RoomCount returns the number of rooms currently tracked. Test/observability only.
func (b *Broker) RoomCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return len(b.rooms)
}

// PeerCount returns how many peers (0, 1, or 2) are currently attached to the
// room for consultationID. Returns 0 if no such room exists.
func (b *Broker) PeerCount(consultationID uuid.UUID) int {
	room := b.getRoom(consultationID)
	if room == nil {
		return 0
	}
	room.mu.Lock()
	defer room.mu.Unlock()

	return len(room.peers)
}
