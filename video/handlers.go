package video

import (
	"context"
	"cybermedico/cmd/cybermedico/api/log"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// wsReadLimit caps incoming message size. SDP envelopes top out in low single
// kilobytes; ICE candidates are smaller. 64 KiB is generous and shuts the
// door on accidental or malicious large payloads.
const wsReadLimit = 64 << 10

var (
	// ErrJoinUnauthorized means the caller is neither a logged-in doctor nor holds a join token.
	ErrJoinUnauthorized = errors.New("video: not logged in and no join token")
	// ErrJoinForbidden means the caller is identified but may not join this room now.
	ErrJoinForbidden = errors.New("video: not allowed to join this call")
)

// AuthorizeFunc decides which side of the call a request may join. It returns
// ErrJoinUnauthorized or ErrJoinForbidden to refuse, so the handler can answer
// with the matching status before upgrading the connection.
type AuthorizeFunc func(r *http.Request, consultationID uuid.UUID) (Role, error)

// EventRecorder persists call lifecycle events for audit purposes. nil values
// are treated as "no recording" by handlers.
type EventRecorder interface {
	Record(ctx context.Context, consultationID uuid.UUID, kind string, detail map[string]any) error
}

// noopRecorder is the zero-value EventRecorder used when none is configured.
type noopRecorder struct{}

func (noopRecorder) Record(context.Context, uuid.UUID, string, map[string]any) error { return nil }

// PGEventRecorder writes events into the consultation_call_events table.
type PGEventRecorder struct {
	pool *pgxpool.Pool
}

// NewPGEventRecorder builds a recorder backed by pool.
func NewPGEventRecorder(pool *pgxpool.Pool) *PGEventRecorder {
	return &PGEventRecorder{pool: pool}
}

// Record inserts one row. detail is marshaled to JSONB; if it's nil, the
// detail column is set to NULL.
func (r *PGEventRecorder) Record(ctx context.Context, consultationID uuid.UUID, kind string, detail map[string]any) error {
	var detailJSON []byte
	if len(detail) > 0 {
		b, err := json.Marshal(detail)
		if err != nil {
			return err
		}
		detailJSON = b
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO consultation_call_events (consultation_id, kind, detail)
		VALUES ($1, $2, $3)
	`, consultationID, kind, detailJSON)

	return err
}

// SignalingHandler upgrades GET requests to a WebSocket and joins the peer
// to the broker for its consultation room. URL pattern must include {id} as
// a path value (Go 1.22+ ServeMux pattern).
type SignalingHandler struct {
	broker    *Broker
	authorize AuthorizeFunc
	recorder  EventRecorder
	logger    *log.LoggerType
}

// NewSignalingHandler constructs a SignalingHandler. recorder may be nil.
func NewSignalingHandler(broker *Broker, authorize AuthorizeFunc, recorder EventRecorder) *SignalingHandler {
	if recorder == nil {
		recorder = noopRecorder{}
	}

	return &SignalingHandler{
		broker:    broker,
		authorize: authorize,
		recorder:  recorder,
		logger:    log.Logger,
	}
}

func (h *SignalingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseConsultationID(w, r)
	if !ok {
		return
	}

	// Checked before the upgrade, so a refused caller never gets a connection.
	role, err := h.authorize(r, cid)
	switch {
	case errors.Is(err, ErrJoinUnauthorized):
		http.Error(w, "unauthorized", http.StatusUnauthorized)

		return
	case errors.Is(err, ErrJoinForbidden):
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	case err != nil:
		h.logger.Error("video signaling: authorization failed", "error", err, "consultation_id", cid)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// No OriginPatterns: the default accepts only pages from the app's own host,
	// so another site can't open this socket with a doctor's cookie.
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		h.logger.Warn("video signaling: websocket accept failed",
			"error", err,
			"consultation_id", cid,
			"role", role,
		)

		return
	}
	conn.SetReadLimit(wsReadLimit)

	peer := NewWSPeer(conn)
	defer peer.Close()

	if err := h.broker.Join(cid, role, peer); err != nil {
		h.logger.Error("video signaling: broker join failed",
			"error", err,
			"consultation_id", cid,
			"role", role,
		)

		return
	}
	defer h.broker.Leave(cid, role, peer)

	joinKind, leaveKind := lifecycleEventKinds(role)
	h.recordEvent(cid, joinKind, nil)
	defer h.recordEvent(cid, leaveKind, nil)

	h.readLoop(r.Context(), conn, cid, role)
}

// readLoop pulls messages off the websocket and hands them to the broker for
// relay. Returns when the connection closes or the request context is
// cancelled.
func (h *SignalingHandler) readLoop(ctx context.Context, conn *websocket.Conn, cid uuid.UUID, role Role) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			h.logger.Debug("video signaling: malformed message",
				"error", err,
				"consultation_id", cid,
				"role", role,
			)

			continue
		}

		if !isClientMessageType(msg.Type) {
			h.logger.Debug("video signaling: rejected message type",
				"type", msg.Type,
				"consultation_id", cid,
				"role", role,
			)

			continue
		}

		h.broker.Relay(cid, role, msg)
	}
}

// recordEvent enqueues an event write on a background goroutine so DB
// latency does not block the signaling read loop. Uses a detached context
// with a short timeout — events are best-effort and not worth retrying.
func (h *SignalingHandler) recordEvent(cid uuid.UUID, kind string, detail map[string]any) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.recorder.Record(ctx, cid, kind, detail); err != nil {
			h.logger.Warn("video signaling: failed to record event",
				"error", err,
				"consultation_id", cid,
				"kind", kind,
			)
		}
	}()
}

// parseConsultationID reads {id} from the Go 1.22+ ServeMux path value.
// Writes a 400 to w and returns false if the value is missing or malformed.
func parseConsultationID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "missing consultation id", http.StatusBadRequest)

		return uuid.Nil, false
	}
	cid, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid consultation id", http.StatusBadRequest)

		return uuid.Nil, false
	}

	return cid, true
}

// isClientMessageType reports whether t is a message a client is permitted to
// send to the broker. Broker-emitted notifications (peer-joined, peer-left,
// peer-rejoined) are not valid for inbound use.
func isClientMessageType(t MessageType) bool {
	switch t {
	case MsgOffer, MsgAnswer, MsgICECandidate, MsgBye:
		return true
	}

	return false
}

// lifecycleEventKinds returns (joinKind, leaveKind) for role.
func lifecycleEventKinds(role Role) (string, string) {
	if role == RoleDoctor {
		return "doctor_joined", "doctor_left"
	}

	return "patient_joined", "patient_left"
}
