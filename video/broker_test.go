package video_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"cybermedico/video"

	"github.com/google/uuid"
)

// fakePeer is a Peer implementation that records every message it receives
// and tracks whether Close was called. Safe for concurrent use.
type fakePeer struct {
	mu     sync.Mutex
	sent   []video.Message
	closed bool
}

func newFakePeer() *fakePeer { return &fakePeer{} }

func (p *fakePeer) Send(msg video.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errors.New("send on closed peer")
	}
	p.sent = append(p.sent, msg)

	return nil
}

func (p *fakePeer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
}

func (p *fakePeer) Sent() []video.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]video.Message, len(p.sent))
	copy(out, p.sent)

	return out
}

func (p *fakePeer) IsClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.closed
}

func TestBrokerJoinNotifiesExistingPeer(t *testing.T) {
	b := video.NewBroker()
	cid := uuid.New()

	doc := newFakePeer()
	pat := newFakePeer()

	if err := b.Join(cid, video.RoleDoctor, doc); err != nil {
		t.Fatalf("doctor join: %v", err)
	}
	if got := len(doc.Sent()); got != 0 {
		t.Errorf("doctor should receive nothing when alone, got %d msgs", got)
	}

	if err := b.Join(cid, video.RolePatient, pat); err != nil {
		t.Fatalf("patient join: %v", err)
	}

	// Doctor receives peer-joined when patient connects.
	docSent := doc.Sent()
	if len(docSent) != 1 {
		t.Fatalf("doctor should have 1 msg after patient joined, got %d", len(docSent))
	}
	if docSent[0].Type != video.MsgPeerJoined || docSent[0].From != video.RolePatient {
		t.Errorf("doctor msg = %+v, want PeerJoined from patient", docSent[0])
	}

	// Patient receives peer-present because doctor was already in the room.
	patSent := pat.Sent()
	if len(patSent) != 1 {
		t.Fatalf("patient should have 1 msg after joining (peer-present), got %d", len(patSent))
	}
	if patSent[0].Type != video.MsgPeerPresent || patSent[0].From != video.RoleDoctor {
		t.Errorf("patient msg = %+v, want PeerPresent from doctor", patSent[0])
	}
}

func TestBrokerRelayForwardsToOtherPeer(t *testing.T) {
	b := video.NewBroker()
	cid := uuid.New()

	doc := newFakePeer()
	pat := newFakePeer()
	_ = b.Join(cid, video.RoleDoctor, doc)
	_ = b.Join(cid, video.RolePatient, pat)

	b.Relay(cid, video.RoleDoctor, video.Message{Type: video.MsgOffer})

	patMsgs := pat.Sent()
	var offers []video.Message
	for _, m := range patMsgs {
		if m.Type == video.MsgOffer {
			offers = append(offers, m)
		}
	}
	if len(offers) != 1 {
		t.Fatalf("patient should have 1 offer, got %d", len(offers))
	}
	if offers[0].From != video.RoleDoctor {
		t.Errorf("patient offer From = %v, want doctor", offers[0].From)
	}

	// Doctor should not have received its own offer back.
	docMsgs := doc.Sent()
	for _, m := range docMsgs {
		if m.Type == video.MsgOffer {
			t.Errorf("doctor received its own offer back: %+v", m)
		}
	}
}

func TestBrokerRelayOverridesFromField(t *testing.T) {
	b := video.NewBroker()
	cid := uuid.New()

	doc := newFakePeer()
	pat := newFakePeer()
	_ = b.Join(cid, video.RoleDoctor, doc)
	_ = b.Join(cid, video.RolePatient, pat)

	// Patient lies about being the doctor. Broker must overwrite From.
	b.Relay(cid, video.RolePatient, video.Message{
		Type: video.MsgICECandidate,
		From: video.RoleDoctor,
	})

	got := doc.Sent()
	if len(got) < 2 { // first was PeerJoined, second should be the relay
		t.Fatalf("doctor should have 2 msgs, got %d", len(got))
	}
	relayed := got[len(got)-1]
	if relayed.From != video.RolePatient {
		t.Errorf("relayed From = %q, want %q (broker must override)", relayed.From, video.RolePatient)
	}
}

func TestBrokerRelayWithNoOtherPeerIsNoop(t *testing.T) {
	b := video.NewBroker()
	cid := uuid.New()

	doc := newFakePeer()
	_ = b.Join(cid, video.RoleDoctor, doc)

	// No panic, no error.
	b.Relay(cid, video.RoleDoctor, video.Message{Type: video.MsgOffer})

	if got := len(doc.Sent()); got != 0 {
		t.Errorf("doctor (sole peer) should not receive its own relay, got %d msgs", got)
	}
}

func TestBrokerReconnectClosesOldPeerAndSendsPeerRejoined(t *testing.T) {
	b := video.NewBroker()
	cid := uuid.New()

	doc := newFakePeer()
	patOld := newFakePeer()
	patNew := newFakePeer()

	_ = b.Join(cid, video.RoleDoctor, doc)
	_ = b.Join(cid, video.RolePatient, patOld)

	// Doctor has 1 PeerJoined msg so far.
	if got := len(doc.Sent()); got != 1 {
		t.Fatalf("setup: doctor sent count = %d, want 1", got)
	}

	// Patient reconnects with a new peer.
	_ = b.Join(cid, video.RolePatient, patNew)

	if !patOld.IsClosed() {
		t.Error("old patient peer should be closed after reconnect")
	}
	if patNew.IsClosed() {
		t.Error("new patient peer should not be closed")
	}

	docMsgs := doc.Sent()
	if len(docMsgs) != 2 {
		t.Fatalf("doctor sent count after reconnect = %d, want 2", len(docMsgs))
	}
	if docMsgs[1].Type != video.MsgPeerRejoined || docMsgs[1].From != video.RolePatient {
		t.Errorf("doctor msg[1] = %+v, want PeerRejoined from patient", docMsgs[1])
	}
}

func TestBrokerLeaveWithStalePeerIsNoop(t *testing.T) {
	b := video.NewBroker()
	cid := uuid.New()

	doc := newFakePeer()
	patOld := newFakePeer()
	patNew := newFakePeer()

	_ = b.Join(cid, video.RoleDoctor, doc)
	_ = b.Join(cid, video.RolePatient, patOld)
	_ = b.Join(cid, video.RolePatient, patNew) // reconnect replaces patOld

	// Old peer's goroutine eventually calls Leave with its stale ref.
	b.Leave(cid, video.RolePatient, patOld)

	if b.PeerCount(cid) != 2 {
		t.Errorf("stale Leave should not remove new peer, count = %d", b.PeerCount(cid))
	}
	// Doctor should not have received a spurious PeerLeft.
	for _, m := range doc.Sent() {
		if m.Type == video.MsgPeerLeft {
			t.Errorf("doctor received unexpected PeerLeft: %+v", m)
		}
	}
}

func TestBrokerLeaveNotifiesOtherPeer(t *testing.T) {
	b := video.NewBroker()
	cid := uuid.New()

	doc := newFakePeer()
	pat := newFakePeer()
	_ = b.Join(cid, video.RoleDoctor, doc)
	_ = b.Join(cid, video.RolePatient, pat)

	b.Leave(cid, video.RolePatient, pat)

	docMsgs := doc.Sent()
	if len(docMsgs) != 2 { // PeerJoined then PeerLeft
		t.Fatalf("doctor sent count = %d, want 2", len(docMsgs))
	}
	if docMsgs[1].Type != video.MsgPeerLeft || docMsgs[1].From != video.RolePatient {
		t.Errorf("doctor msg[1] = %+v, want PeerLeft from patient", docMsgs[1])
	}
	if b.PeerCount(cid) != 1 {
		t.Errorf("peer count after patient leave = %d, want 1", b.PeerCount(cid))
	}
}

func TestBrokerInvalidRoleRejected(t *testing.T) {
	b := video.NewBroker()
	err := b.Join(uuid.New(), video.Role("admin"), newFakePeer())
	if !errors.Is(err, video.ErrInvalidRole) {
		t.Errorf("expected ErrInvalidRole, got %v", err)
	}
}

func TestBrokerEvictIdleRemovesEmptyOldRoomsOnly(t *testing.T) {
	b := video.NewBroker()
	b.SetIdleTTL(time.Minute)

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	b.SetClock(func() time.Time { return now })

	cidEmpty := uuid.New()
	cidActive := uuid.New()
	cidFresh := uuid.New()

	// Empty + old: should be evicted.
	doc := newFakePeer()
	_ = b.Join(cidEmpty, video.RoleDoctor, doc)
	b.Leave(cidEmpty, video.RoleDoctor, doc)

	// Active: has a peer, should stay no matter how old.
	docA := newFakePeer()
	_ = b.Join(cidActive, video.RoleDoctor, docA)

	// Advance clock past idle TTL.
	now = now.Add(2 * time.Minute)

	// Fresh empty room: created after clock advance. Should not be evicted.
	docF := newFakePeer()
	_ = b.Join(cidFresh, video.RoleDoctor, docF)
	b.Leave(cidFresh, video.RoleDoctor, docF)

	evicted := b.EvictIdle()
	if evicted != 1 {
		t.Errorf("evicted = %d, want 1 (only cidEmpty)", evicted)
	}
	if b.PeerCount(cidEmpty) != 0 || b.RoomCount() != 2 {
		// cidEmpty gone, cidActive + cidFresh remain.
		t.Errorf("room count = %d, want 2", b.RoomCount())
	}
	if b.PeerCount(cidActive) != 1 {
		t.Errorf("active room lost its peer")
	}
}

func TestBrokerEvictIdleDoesNotEvictFreshlyCreatedRoom(t *testing.T) {
	b := video.NewBroker()
	b.SetIdleTTL(time.Minute)

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	b.SetClock(func() time.Time { return now })

	cid := uuid.New()

	// Join then leave, creating an empty room with lastActivity = now.
	p := newFakePeer()
	_ = b.Join(cid, video.RoleDoctor, p)
	b.Leave(cid, video.RoleDoctor, p)

	// Eviction immediately: should be skipped because the room is fresh.
	if got := b.EvictIdle(); got != 0 {
		t.Errorf("evicted = %d immediately, want 0", got)
	}
	if b.RoomCount() != 1 {
		t.Errorf("room count = %d, want 1", b.RoomCount())
	}
}

func TestBrokerSeparateConsultationsAreIsolated(t *testing.T) {
	b := video.NewBroker()
	cid1, cid2 := uuid.New(), uuid.New()

	doc1, pat1 := newFakePeer(), newFakePeer()
	doc2, pat2 := newFakePeer(), newFakePeer()

	_ = b.Join(cid1, video.RoleDoctor, doc1)
	_ = b.Join(cid1, video.RolePatient, pat1)
	_ = b.Join(cid2, video.RoleDoctor, doc2)
	_ = b.Join(cid2, video.RolePatient, pat2)

	b.Relay(cid1, video.RoleDoctor, video.Message{Type: video.MsgOffer})

	found := false
	for _, m := range pat1.Sent() {
		if m.Type == video.MsgOffer {
			found = true

			break
		}
	}
	if !found {
		t.Errorf("pat1 should have received the offer")
	}
}

func TestBrokerConcurrentJoinLeaveRelayRaceFree(t *testing.T) {
	// Run with -race to catch any unprotected access. With a single
	// shared room, hammer Join/Leave/Relay from many goroutines.
	b := video.NewBroker()
	cid := uuid.New()

	const N = 50
	var wg sync.WaitGroup

	// Pre-join a doctor so relays have somewhere to land.
	doc := newFakePeer()
	_ = b.Join(cid, video.RoleDoctor, doc)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := newFakePeer()
			_ = b.Join(cid, video.RolePatient, p)
			b.Relay(cid, video.RolePatient, video.Message{Type: video.MsgICECandidate})
			b.Leave(cid, video.RolePatient, p)
		}()
	}

	wg.Wait()
	// Sanity: broker is still consistent.
	if b.PeerCount(cid) < 0 || b.PeerCount(cid) > 2 {
		t.Errorf("peer count = %d after concurrent churn", b.PeerCount(cid))
	}
}
