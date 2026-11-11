// CyberMedico WebRTC client.
//
// One file, no bundler. Reads config from data-* attributes on #video-call
// (see VideoCallView in cmd/cybermedico/api/html/video.templ).
//
// Architecture: 1:1 between doctor and patient. The Go signaling broker only
// relays JSON envelopes; media flows P2P (or via TURN). See
// cmd/cybermedico/video/broker.go for the relay contract.
//
// Negotiation model: asymmetric, not symmetric perfect-negotiation. The
// doctor is the impolite initiator; the patient is the polite answerer. The
// doctor only creates an offer AFTER the patient has joined the room (i.e.
// after a peer-joined signal arrives), so we never get into the glare state
// where both sides have a stale local offer with nothing to relay it to.

const STATUS = {
  connecting: { text: "Connecting...", state: "" },
  waitingPeer: { text: "Waiting for the other side to join...", state: "" },
  connected: { text: "Connected", state: "connected" },
  reconnecting: { text: "Reconnecting...", state: "" },
  peerLeft: { text: "The other side left the call", state: "error" },
  ended: { text: "Call has ended", state: "error" },
  replaced: {
    text: "The call was opened in another window. Continue there.",
    state: "error",
  },
  error: { text: "Error connecting", state: "error" },
  mediaUnavailable: {
    text: "Camera or microphone unavailable",
    state: "error",
  },
  earlyLoad: {
    text: "The call is not open yet. Join closer to the scheduled time.",
    state: "error",
  },
};

const LOG_PREFIX = "[video-call]";
function log(...args) {
  console.log(LOG_PREFIX, ...args);
}
function warn(...args) {
  console.warn(LOG_PREFIX, ...args);
}
function err(...args) {
  console.error(LOG_PREFIX, ...args);
}

class VideoCall {
  constructor(root) {
    this.root = root;
    this.cfg = readConfig(root);
    this.polite = this.cfg.role === "patient";
    this.localStream = null;
    this.pc = null;
    this.ws = null;
    this.peerConnected = false;
    this.peerSaidBye = false;
    this.iceUp = false;
    this.makingOffer = false;
    this.ignoreOffer = false;
    this.hangingUp = false;
    this.reconnectAttempts = 0;
    this.reconnectTimer = null;

    this.statusEl = root.querySelector("#call-status");
    this.mediaHelp = root.querySelector("#media-help");
    this.localVid = root.querySelector("#local-video");
    this.remoteVid = root.querySelector("#remote-video");
    this.micBtn = root.querySelector("#mic-btn");
    this.camBtn = root.querySelector("#cam-btn");
    this.hangupBtn = root.querySelector("#hangup-btn");

    this.micBtn.addEventListener("click", () =>
      this.toggleTrack("audio", this.micBtn, "microphone"),
    );
    this.camBtn.addEventListener("click", () =>
      this.toggleTrack("video", this.camBtn, "camera"),
    );
    this.hangupBtn.addEventListener("click", () => this.hangup());

    window.addEventListener("beforeunload", () => this.hangup());
  }

  setStatus(key) {
    const s = STATUS[key] || STATUS.error;
    this.statusEl.textContent = s.text;
    this.statusEl.dataset.state = s.state;
    log("status →", key);
  }

  async start() {
    this.setStatus("connecting");
    log("role:", this.cfg.role, "consultation:", this.cfg.consultationID);

    // mediaDevices is missing outside a secure context (https or localhost)
    // and in browsers without WebRTC.
    if (!navigator.mediaDevices?.getUserMedia || !window.RTCPeerConnection) {
      this.showMediaHelp("unsupported");
      return;
    }

    try {
      this.localStream = await navigator.mediaDevices.getUserMedia({
        audio: true,
        video: { width: { ideal: 1280 }, height: { ideal: 720 } },
      });
    } catch (e) {
      err("getUserMedia failed:", e);
      this.showMediaHelp(mediaErrorReason(e));
      return;
    }
    this.localVid.srcObject = this.localStream;

    let iceConfig;
    try {
      iceConfig = await this.fetchICEConfig();
    } catch (e) {
      err("ICE config fetch failed:", e);
      if (e.status === 410) this.setStatus("earlyLoad");
      else this.setStatus("error");
      return;
    }
    log("ICE config:", iceConfig);

    this.createPeerConnection(iceConfig);
    this.openSignaling();
  }

  // showMediaHelp tells the user why the call can't use their camera or
  // microphone and what to do about it.
  showMediaHelp(reason) {
    this.setStatus("mediaUnavailable");
    for (const el of this.mediaHelp.querySelectorAll("[data-reason]")) {
      el.hidden = el.dataset.reason !== reason;
    }
    this.mediaHelp.hidden = false;
    this.micBtn.disabled = true;
    this.camBtn.disabled = true;
  }

  async fetchICEConfig() {
    // Without an ICE endpoint the browsers only try direct (host) candidates,
    // which is enough when both sides share a network.
    if (!this.cfg.iceURL) return { iceServers: [] };

    const url = withToken(this.cfg.iceURL, this.cfg.token);
    const resp = await fetch(url, { credentials: "same-origin" });
    if (!resp.ok) {
      const body = await resp.text().catch(() => "");
      const e = new Error(`ICE fetch ${resp.status}: ${body.trim()}`);
      e.status = resp.status;
      e.body = body.trim();
      throw e;
    }
    return resp.json();
  }

  createPeerConnection(iceConfig) {
    const pc = new RTCPeerConnection(iceConfig);
    this.pc = pc;

    for (const track of this.localStream.getTracks()) {
      pc.addTrack(track, this.localStream);
    }

    pc.ontrack = (ev) => {
      // First stream is the remote peer. WebRTC delivers tracks as
      // they arrive; binding once with the same MediaStream object
      // handles both video and audio.
      if (ev.streams && ev.streams[0]) {
        log("ontrack:", ev.track.kind);
        this.remoteVid.srcObject = ev.streams[0];
      }
    };

    // Polite side (patient) never initiates. Impolite side (doctor) only
    // initiates once a peer has joined; addTrack will queue a needed
    // negotiation that we trigger manually from onSignal.
    pc.onnegotiationneeded = () => {
      log(
        "negotiationneeded; polite=",
        this.polite,
        "peerConnected=",
        this.peerConnected,
      );
      if (!this.polite && this.peerConnected) {
        this.sendOffer().catch((e) => err("sendOffer:", e));
      }
    };

    pc.onicecandidate = ({ candidate }) => {
      if (candidate) {
        this.sendSignal({ type: "ice-candidate", data: candidate });
      }
    };

    pc.oniceconnectionstatechange = () => {
      const s = pc.iceConnectionState;
      log("iceConnectionState:", s);
      if (s === "connected" || s === "completed") {
        this.setStatus("connected");
        // "completed" follows "connected"; report the connection once.
        if (!this.iceUp) {
          this.iceUp = true;
          this.report("ice_connected");
          this.reportRelayUse();
        }
      } else if (s === "failed" && this.peerConnected) {
        this.iceUp = false;
        this.report("ice_failed");
        pc.restartIce();
        this.setStatus("reconnecting");
      } else if (s === "disconnected" && this.peerConnected) {
        this.iceUp = false;
        this.setStatus("reconnecting");
      }
    };

    pc.onconnectionstatechange = () => {
      const s = pc.connectionState;
      log("connectionState:", s);
      if (s === "connected") {
        this.setStatus("connected");
      } else if (s === "failed" && this.peerConnected) {
        this.setStatus("error");
      }
    };
  }

  async sendOffer(options = {}) {
    try {
      this.makingOffer = true;
      const offer = await this.pc.createOffer(options);
      await this.pc.setLocalDescription(offer);
      this.sendSignal({ type: "offer", data: this.pc.localDescription });
      log("offer sent; iceRestart=", !!options.iceRestart);
    } finally {
      this.makingOffer = false;
    }
  }

  openSignaling() {
    const url = withToken(toAbsoluteWS(this.cfg.wsURL), this.cfg.token);
    log("opening WS:", url);
    this.ws = new WebSocket(url);

    this.ws.addEventListener("open", () => {
      log("WS open");
      this.reconnectAttempts = 0;
      this.setStatus("waitingPeer");
    });

    this.ws.addEventListener("message", (ev) => this.onSignal(ev));

    this.ws.addEventListener("close", (ev) => {
      log("WS close:", ev.code, ev.reason);
      if (this.hangingUp) return;
      // 4001: the same side joined from another tab or device and took over.
      // Reconnecting would kick that one out in turn.
      if (ev.code === 4001) {
        this.hangup();
        this.setStatus("replaced");
        return;
      }
      this.setStatus("reconnecting");
      this.scheduleReconnect();
    });

    this.ws.addEventListener("error", (e) => {
      warn("WS error:", e);
    });
  }

  scheduleReconnect() {
    if (this.reconnectAttempts >= 30) {
      this.setStatus("error");
      return;
    }
    const delay = Math.min(10_000, 500 * Math.pow(1.6, this.reconnectAttempts));
    this.reconnectAttempts++;
    this.reconnectTimer = setTimeout(() => this.openSignaling(), delay);
  }

  async onSignal(ev) {
    let msg;
    try {
      msg = JSON.parse(ev.data);
    } catch {
      return;
    }

    const pc = this.pc;
    if (!pc || pc.signalingState === "closed") {
      // Late message after teardown — ignore.
      return;
    }

    log("← signal:", msg.type, msg.from || "");

    try {
      switch (msg.type) {
        case "offer":
        case "answer": {
          const offerCollision =
            msg.type === "offer" &&
            (this.makingOffer || pc.signalingState !== "stable");
          this.ignoreOffer = !this.polite && offerCollision;
          if (this.ignoreOffer) {
            warn("ignored offer due to collision");
            return;
          }

          await pc.setRemoteDescription(msg.data);
          if (msg.type === "offer") {
            await pc.setLocalDescription();
            this.sendSignal({ type: "answer", data: pc.localDescription });
            log("answer sent");
          } else {
            // answer received — negotiation complete, peer is (re)connected
            if (
              pc.iceConnectionState === "connected" ||
              pc.iceConnectionState === "completed"
            ) {
              this.setStatus("connected");
            }
          }
          break;
        }
        case "ice-candidate":
          if (!msg.data) break;
          try {
            await pc.addIceCandidate(msg.data);
          } catch (e) {
            if (!this.ignoreOffer && pc.signalingState !== "closed") {
              warn("addIceCandidate failed:", e.message);
            }
          }
          break;
        case "peer-present":
        case "peer-joined":
          clearTimeout(this.peerLeftTimer);
          this.peerConnected = true;
          this.peerSaidBye = false;
          if (!this.polite) await this.sendOffer();
          break;
        case "peer-rejoined":
          clearTimeout(this.peerLeftTimer);
          this.peerConnected = true;
          this.peerSaidBye = false;
          if (!this.polite) {
            // Fresh offer with ICE restart so the new transport
            // path is discovered.
            await this.sendOffer({ iceRestart: true });
          }
          break;
        case "peer-left":
        case "bye":
          this.peerConnected = false;
          if (msg.type === "bye") {
            // Explicit hangup — they're not coming back
            this.peerSaidBye = true;
            this.setStatus("peerLeft");
          } else if (!this.peerSaidBye) {
            // Unclean drop — they may reconnect, show softer status.
            // Skipped after a bye: the broker's peer-left that follows it
            // must not turn "left the call" back into "reconnecting".
            this.setStatus("reconnecting");
            this.peerLeftTimer = setTimeout(() => {
              if (!this.peerConnected) this.setStatus("peerLeft");
            }, 15_000);
          }
          break;
      }
    } catch (e) {
      err("onSignal:", e);
    }
  }

  // report tells the server how the connection went, for the consultation's
  // call log. The server records it and doesn't pass it on.
  report(kind) {
    this.sendSignal({ type: "report", data: { kind } });
  }

  // reportRelayUse reports when media goes through the TURN server, i.e. a
  // direct connection wasn't possible.
  async reportRelayUse() {
    try {
      const stats = await this.pc.getStats();
      let pair = null;
      stats.forEach((r) => {
        if (r.type === "transport" && r.selectedCandidatePairId) {
          pair = stats.get(r.selectedCandidatePairId);
        }
      });
      if (!pair) {
        stats.forEach((r) => {
          if (r.type === "candidate-pair" && r.nominated && r.state === "succeeded") {
            pair = r;
          }
        });
      }
      if (!pair) return;
      const local = stats.get(pair.localCandidateId);
      const remote = stats.get(pair.remoteCandidateId);
      if (local?.candidateType === "relay" || remote?.candidateType === "relay") {
        this.report("ice_relay_used");
      }
    } catch (e) {
      warn("getStats:", e);
    }
  }

  sendSignal(msg) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      warn("drop signal (WS not open):", msg.type);
      return;
    }
    this.ws.send(JSON.stringify(msg));
  }

  toggleTrack(kind, btn, labelNoun) {
    if (!this.localStream) return;
    const tracks =
      kind === "audio"
        ? this.localStream.getAudioTracks()
        : this.localStream.getVideoTracks();
    if (tracks.length === 0) return;

    const wasEnabled = tracks[0].enabled;
    for (const t of tracks) t.enabled = !wasEnabled;
    const enabled = !wasEnabled;
    btn.setAttribute("aria-pressed", String(!enabled));
    btn.textContent = enabled ? `Turn off ${labelNoun}` : `Turn on ${labelNoun}`;
  }

  hangup() {
    if (this.hangingUp) return;
    this.hangingUp = true;
    try {
      this.sendSignal({ type: "bye" });
    } catch {}
    this.cleanup();
    this.setStatus("ended");
    this.micBtn.disabled = true;
    this.camBtn.disabled = true;
    this.hangupBtn.disabled = true;
  }

  cleanup() {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    try {
      if (this.ws) this.ws.close();
    } catch {}
    try {
      if (this.pc) this.pc.close();
    } catch {}
    if (this.localStream) {
      for (const t of this.localStream.getTracks()) t.stop();
    }
  }
}

// mediaErrorReason maps a getUserMedia error to the matching help text.
function mediaErrorReason(e) {
  switch (e && e.name) {
    case "NotAllowedError":
    case "SecurityError":
      return "denied";
    case "TypeError":
      return "unsupported";
    default:
      // NotFoundError, NotReadableError (device busy), OverconstrainedError.
      return "missing";
  }
}

function readConfig(root) {
  return {
    role: root.dataset.role,
    consultationID: root.dataset.consultationId,
    token: root.dataset.token || "",
    scheduledISO: root.dataset.scheduledAt || "",
    wsURL: root.dataset.wsUrl,
    iceURL: root.dataset.iceUrl || "",
  };
}

function withToken(url, token) {
  if (!token) return url;
  const sep = url.includes("?") ? "&" : "?";
  return url + sep + "t=" + encodeURIComponent(token);
}

function toAbsoluteWS(path) {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  if (path.startsWith("ws://") || path.startsWith("wss://")) return path;
  return proto + "//" + location.host + path;
}

const root = document.querySelector("#video-call");
if (root) {
  const call = new VideoCall(root);
  call.start();
}
