package html

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // TURN REST API mandates HMAC-SHA1
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/api/util"
	api "cybermedico/cmd/cybermedico/ogen"
	"cybermedico/video"
)

type TURNConfig struct {
	// Hostname is the public hostname of the coturn server, e.g. "turn.cybermedico.net".
	Hostname string
	// SharedSecret matches coturn's static-auth-secret. Used to mint per-session
	// HMAC-SHA1 credentials following the long-term credential mechanism with
	// the "use-auth-secret" extension (a.k.a. REST API for TURN).
	SharedSecret []byte
	// CredentialTTL controls how long minted credentials remain valid.
	// 2h is generous: covers the 90-minute join window plus reconnect slack.
	CredentialTTL time.Duration
	// STUNURLs is an optional list of additional STUN servers to advertise
	// to the browser. coturn itself answers STUN, so this is usually empty.
	STUNURLs []string
}

// ICEHandler returns the iceServers JSON config the browser needs to
// construct an RTCPeerConnection. The TURN credentials are short-lived
// HMACs minted from a shared secret coturn also knows.
type ICEHandler struct {
	turn *TURNConfig
	// authorize is the same check as for the signaling socket: TURN credentials
	// let anyone relay traffic through our server, so only the two sides of a
	// call that is open right now get them.
	authorize video.AuthorizeFunc
}

type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// ICEConfig is the response shape for the /api/video/{id}/ice endpoint.
type ICEConfig struct {
	ICEServers []ICEServer `json:"iceServers"`
}

// NewICEHandler wires an ICEHandler. turn may be nil: the browsers then only try
// direct connections, which is enough when both sides share a network.
func NewICEHandler(turn *TURNConfig, authorize video.AuthorizeFunc) ICEHandler {
	return ICEHandler{turn: turn, authorize: authorize}
}

func (h ICEHandler) VideoPageIce(ctx context.Context, params api.VideoPageIceParams) (api.VideoPageIceRes, error) {
	_, err := h.authorize(util.RequestFromContext(ctx), params.ID)
	switch {
	case errors.Is(err, video.ErrJoinUnauthorized):
		return &api.VideoPageIceUnauthorized{Error: "unauthorized"}, nil
	case errors.Is(err, video.ErrJoinForbidden):
		return &api.VideoPageIceForbidden{Error: "forbidden"}, nil
	case err != nil:
		return nil, fmt.Errorf("failed to authorize ICE request: %w", err)
	}

	// Credentials are minted per request and must not be cached anywhere.
	util.ResponseFromContext(ctx).Header().Set("Cache-Control", "no-store")

	if h.turn == nil {
		return &api.ICEConfig{IceServers: []api.ICEServer{}}, nil
	}

	raw, err := issueICEConfig(*h.turn, params.ID.String(), time.Now())
	if err != nil {
		log.Logger.Error("failed to issue ICE config", "error", err, "consultation_id", params.ID.String())

		return &api.VideoPageIceInternalServerError{Error: "failed to issue ICE config"}, nil
	}

	servers := make([]api.ICEServer, len(raw))
	for i, s := range raw {
		servers[i] = api.ICEServer{
			Urls:       s.URLs,
			Username:   api.NewOptString(s.Username),
			Credential: api.NewOptString(s.Credential),
		}
	}

	return &api.ICEConfig{IceServers: servers}, nil
}

func issueICEConfig(cfg TURNConfig, sessionID string, now time.Time) ([]ICEServer, error) {
	if cfg.Hostname == "" {
		return nil, errors.New("video: TURN hostname is empty")
	}
	if len(cfg.SharedSecret) == 0 {
		return nil, errors.New("video: TURN shared secret is empty")
	}
	if cfg.CredentialTTL <= 0 {
		return nil, errors.New("video: TURN credential TTL must be positive")
	}

	expiry := now.Add(cfg.CredentialTTL).Unix()
	username := fmt.Sprintf("%d:%s", expiry, sessionID)

	mac := hmac.New(sha1.New, cfg.SharedSecret)
	mac.Write([]byte(username))
	credential := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	turn := ICEServer{
		URLs: []string{
			fmt.Sprintf("turn:%s:3478?transport=udp", cfg.Hostname),
			fmt.Sprintf("turn:%s:3478?transport=tcp", cfg.Hostname),
			fmt.Sprintf("turns:%s:5349?transport=tcp", cfg.Hostname),
			// 443 over TLS is the escape hatch for restrictive corporate firewalls.
			fmt.Sprintf("turns:%s:443?transport=tcp", cfg.Hostname),
		},
		Username:   username,
		Credential: credential,
	}

	servers := []ICEServer{turn}
	if len(cfg.STUNURLs) > 0 {
		servers = append([]ICEServer{{URLs: cfg.STUNURLs}}, servers...)
	}

	return servers, nil
}
