package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/authrelay"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/pkg/authmetrics"
)

type AuthRelay struct {
	relay           *authrelay.Relay
	tokenHash       [32]byte
	onSession       func(client, reference, sid, sidtime string) string
	explicitSIDTime bool
}

func NewAuthRelay(relay *authrelay.Relay, token string) *AuthRelay {
	return &AuthRelay{relay: relay, tokenHash: sha256.Sum256([]byte(token))}
}

func (r *AuthRelay) OnSession(callback func(client, reference, sid, sidtime string) string, explicitSIDTime bool) {
	r.onSession = callback
	r.explicitSIDTime = explicitSIDTime
}

var explicitSID = regexp.MustCompile(`^[a-z0-9]{1,32}$`)
var explicitSIDTime = regexp.MustCompile(`^[1-9][0-9]{0,8}$`)

func (r *AuthRelay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	isMetrics := req.Method == http.MethodGet && req.URL.Path == "/v1/gateway/auth/metrics"
	if !isMetrics && (req.Method != http.MethodPost || req.URL.Path != "/v1/gateway/auth") {
		http.NotFound(w, req)
		return
	}
	scheme, token, _ := strings.Cut(req.Header.Get("Authorization"), " ")
	digest := sha256.Sum256([]byte(token))
	if token == "" || !strings.EqualFold(scheme, "Bearer") || subtle.ConstantTimeCompare(digest[:], r.tokenHash[:]) != 1 {
		authmetrics.RecordDecision("LOCAL_TOKEN_DENIED")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if isMetrics {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(authmetrics.Counters.String()))
		return
	}
	started := time.Now()
	defer authmetrics.RecordLatency("authenticate", started)
	var input struct{ Username, Password, Client string }
	decoder := json.NewDecoder(http.MaxBytesReader(w, req.Body, 16<<10))
	if err := decoder.Decode(&input); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	base, sidtime, hasSIDTime := strings.Cut(input.Username, "-sidtime-")
	username, sid, explicit := strings.Cut(base, "-sid-")
	id, reference, reason := "", "", "INVALID_CREDENTIAL"
	if (!hasSIDTime || r.onSession != nil && r.explicitSIDTime && explicitSIDTime.MatchString(sidtime)) &&
		(!explicit || r.onSession != nil && explicitSID.MatchString(sid)) {
		id, reference, reason = r.relay.AuthenticateTenant(req.Context(), username, input.Password, input.Client)
	}
	if id != "" && r.onSession != nil {
		id = r.onSession(id, reference, sid, sidtime)
	}
	authmetrics.RecordDecision(reason)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(struct {
		OK bool   `json:"ok"`
		ID string `json:"id,omitempty"`
	}{id != "", id})
}
