package video_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"cybermedico/video"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// fakeRecorder records every Record call. Safe for concurrent use.
type fakeRecorder struct {
	mu    sync.Mutex
	kinds []string
}

func (r *fakeRecorder) Record(_ context.Context, _ uuid.UUID, kind string, _ map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kinds = append(r.kinds, kind)

	return nil
}

func (r *fakeRecorder) Kinds() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.kinds...)
}

// fakeAuthorize stands in for the real check: the X-Test-Join header names the
// role to grant, or "forbidden". Without it the caller is unauthorized.
func fakeAuthorize(r *http.Request, _ uuid.UUID) (video.Role, error) {
	switch r.Header.Get("X-Test-Join") {
	case "doctor":
		return video.RoleDoctor, nil
	case "patient":
		return video.RolePatient, nil
	case "forbidden":
		return "", video.ErrJoinForbidden
	}

	return "", video.ErrJoinUnauthorized
}

type signalingTestRig struct {
	srv      *httptest.Server
	recorder *fakeRecorder
	cid      uuid.UUID
}

func newSignalingRig(t *testing.T) *signalingTestRig {
	t.Helper()
	recorder := &fakeRecorder{}
	handler := video.NewSignalingHandler(video.NewBroker(), fakeAuthorize, recorder)

	mux := http.NewServeMux()
	mux.Handle("/ws/video/{id}/signal", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &signalingTestRig{srv: srv, recorder: recorder, cid: uuid.New()}
}

func (rig *signalingTestRig) url() string {
	return "ws" + strings.TrimPrefix(rig.srv.URL, "http") + "/ws/video/" + rig.cid.String() + "/signal"
}

func dial(rig *signalingTestRig, headers http.Header) (*websocket.Conn, *http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return websocket.Dial(ctx, rig.url(), &websocket.DialOptions{HTTPHeader: headers})
}

func join(t *testing.T, rig *signalingTestRig, role string) *websocket.Conn {
	t.Helper()
	conn, _, err := dial(rig, http.Header{"X-Test-Join": []string{role}})
	if err != nil {
		t.Fatalf("dial as %s: %v", role, err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	return conn
}

func readMessage(t *testing.T, conn *websocket.Conn) video.Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg video.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	return msg
}

func writeMessage(t *testing.T, conn *websocket.Conn, msg video.Message) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestSignalingDoctorAndPatientRelay(t *testing.T) {
	rig := newSignalingRig(t)
	doctor := join(t, rig, "doctor")
	patient := join(t, rig, "patient")

	if got := readMessage(t, doctor); got.Type != video.MsgPeerJoined || got.From != video.RolePatient {
		t.Fatalf("doctor: expected PeerJoined from patient, got %+v", got)
	}
	if got := readMessage(t, patient); got.Type != video.MsgPeerPresent {
		t.Fatalf("patient: expected PeerPresent, got %+v", got)
	}

	writeMessage(t, doctor, video.Message{Type: video.MsgOffer, Data: json.RawMessage(`"sdp-bytes"`)})
	got := readMessage(t, patient)
	if got.Type != video.MsgOffer || got.From != video.RoleDoctor || string(got.Data) != `"sdp-bytes"` {
		t.Fatalf("patient: expected Offer from doctor, got %+v", got)
	}

	// A client can't pretend to be the other side: the broker stamps From.
	writeMessage(t, patient, video.Message{Type: video.MsgAnswer, From: video.RoleDoctor, Data: json.RawMessage(`"answer"`)})
	got = readMessage(t, doctor)
	if got.Type != video.MsgAnswer || got.From != video.RolePatient {
		t.Fatalf("doctor: expected Answer from patient, got %+v", got)
	}
}

func TestSignalingRejectsBrokerOnlyMessageTypes(t *testing.T) {
	rig := newSignalingRig(t)
	doctor := join(t, rig, "doctor")
	patient := join(t, rig, "patient")
	_ = readMessage(t, doctor) // PeerJoined

	writeMessage(t, patient, video.Message{Type: video.MsgPeerLeft})
	writeMessage(t, patient, video.Message{Type: video.MsgAnswer, Data: json.RawMessage(`"ok"`)})

	if got := readMessage(t, doctor); got.Type != video.MsgAnswer {
		t.Fatalf("expected Answer (PeerLeft must be dropped), got %+v", got)
	}
}

func TestSignalingRefusesBeforeUpgrade(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		want    int
	}{
		{"no login and no token", nil, http.StatusUnauthorized},
		{"not allowed", http.Header{"X-Test-Join": []string{"forbidden"}}, http.StatusForbidden},
		{"page from another site", http.Header{"X-Test-Join": []string{"doctor"}, "Origin": []string{"https://evil.example"}}, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newSignalingRig(t)
			conn, resp, err := dial(rig, tt.headers)
			if err == nil {
				_ = conn.CloseNow()
				t.Fatal("expected dial failure")
			}
			if resp == nil || resp.StatusCode != tt.want {
				status := 0
				if resp != nil {
					status = resp.StatusCode
				}
				t.Fatalf("status = %d, want %d", status, tt.want)
			}
			if kinds := rig.recorder.Kinds(); len(kinds) != 0 {
				t.Fatalf("recorded %v for a refused caller", kinds)
			}
		})
	}
}

func TestSignalingInvalidConsultationID(t *testing.T) {
	rig := newSignalingRig(t)
	res, err := http.Get(rig.srv.URL + "/ws/video/not-a-uuid/signal")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestSignalingRecordsJoinAndLeaveEvents(t *testing.T) {
	rig := newSignalingRig(t)
	doctor := join(t, rig, "doctor")
	patient := join(t, rig, "patient")
	_ = readMessage(t, doctor) // PeerJoined

	_ = doctor.Close(websocket.StatusNormalClosure, "")
	_ = patient.Close(websocket.StatusNormalClosure, "")

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		kinds := strings.Join(rig.recorder.Kinds(), ",")
		if strings.Contains(kinds, "doctor_joined") && strings.Contains(kinds, "patient_joined") &&
			strings.Contains(kinds, "doctor_left") && strings.Contains(kinds, "patient_left") {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("recorded kinds = %v", rig.recorder.Kinds())
}
