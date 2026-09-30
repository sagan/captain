package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/zeptop-dev/captain/internal/auth"
	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/http/ratelimit"
	"github.com/zeptop-dev/captain/internal/store"
)

const passkeyCookie = "captain_passkey"

type passkeyUser struct {
	principal *domain.User
	handle    []byte
	keys      []store.Passkey
}

func (u *passkeyUser) WebAuthnID() []byte          { return u.handle }
func (u *passkeyUser) WebAuthnName() string        { return u.principal.Email }
func (u *passkeyUser) WebAuthnDisplayName() string { return u.principal.Email }
func (u *passkeyUser) WebAuthnCredentials() []webauthn.Credential {
	out := make([]webauthn.Credential, 0, len(u.keys))
	for _, key := range u.keys {
		out = append(out, key.Credential)
	}
	return out
}
func (h *handlers) passkeyUser(ctx context.Context, u *domain.User, rp string) (*passkeyUser, error) {
	handle, err := h.Store.PasskeyHandle(ctx, u.ID, rp)
	if err != nil {
		return nil, err
	}
	keys, err := h.Store.Passkeys(ctx, u.ID, rp)
	if err != nil {
		return nil, err
	}
	return &passkeyUser{u, handle, keys}, nil
}
func (h *handlers) webAuthn() (*webauthn.WebAuthn, error) {
	u, err := url.Parse(h.BaseURL)
	if err != nil || u.Hostname() == "" || u.User != nil {
		return nil, errors.New("invalid base_url")
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return nil, errors.New("passkeys require HTTPS or localhost")
	}
	timeout := webauthn.TimeoutConfig{Enforce: true, Timeout: 5 * time.Minute, TimeoutUVD: 5 * time.Minute}
	return webauthn.New(&webauthn.Config{RPID: u.Hostname(), RPDisplayName: firstNonEmpty(h.SiteName, "Captain"), RPOrigins: []string{u.Scheme + "://" + u.Host}, AuthenticatorSelection: protocol.AuthenticatorSelection{ResidentKey: protocol.ResidentKeyRequirementRequired, UserVerification: protocol.VerificationRequired}, AttestationPreference: protocol.PreferNoAttestation, Timeouts: webauthn.TimeoutsConfig{Login: timeout, Registration: timeout}})
}
func (h *handlers) registerPasskeys(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/passkeys", h.requireAdmin(h.listPasskeys))
	mux.HandleFunc("POST /api/admin/passkeys/register/begin", h.requireAdmin(h.beginPasskeyRegistration))
	mux.HandleFunc("POST /api/admin/passkeys/register/finish", h.requireAdmin(h.finishPasskeyRegistration))
	mux.HandleFunc("DELETE /api/admin/passkeys/{key}", h.requireAdmin(h.deletePasskey))
	mux.HandleFunc("POST /api/admin/passkeys/login/begin", h.sameOrigin(h.beginPasskeyLogin))
	mux.HandleFunc("POST /api/admin/passkeys/login/finish", h.sameOrigin(h.finishPasskeyLogin))
}
func (h *handlers) passkeyReady(w http.ResponseWriter, r *http.Request, begin bool) (*webauthn.WebAuthn, bool) {
	ip := ratelimit.ClientIP(r)
	if !h.adminAllowed(r.Context(), ip) {
		fail(w, 403, "your address is not on the admin allow-list")
		return nil, false
	}
	if h.Logins != nil {
		if allowed, _ := h.Logins.Allow(ip); !allowed {
			fail(w, 429, "too many failed attempts; try again later")
			return nil, false
		}
	}
	wa, err := h.webAuthn()
	if err != nil {
		fail(w, 503, "passkeys require an HTTPS base_url (or localhost)")
		return nil, false
	}
	// Registration and authentication always bind to configured public origin,
	// never a Host / forwarded-host value supplied by the request.
	if origin := r.Header.Get("Origin"); origin != "" && origin != wa.Config.RPOrigins[0] {
		fail(w, 403, "passkey origin mismatch")
		return nil, false
	}
	if begin {
		if allowed, _ := h.passkeyStarts.Allow(ip); !allowed {
			fail(w, 429, "too many passkey attempts; try again later")
			return nil, false
		}
		h.passkeyStarts.Fail(ip)
	}
	return wa, true
}
func (h *handlers) passkeyFailure(w http.ResponseWriter, r *http.Request) {
	if h.Logins != nil {
		h.Logins.Fail(ratelimit.ClientIP(r))
	}
	fail(w, 401, "passkey verification failed")
}
func (h *handlers) passkeyReauth(w http.ResponseWriter, r *http.Request, password, code string) bool {
	u := userFrom(r)
	if !auth.VerifyPasswordOrDummy(u.PasswordHash, password) {
		h.passkeyFailure(w, r)
		return false
	}
	return h.passkeyTOTP(w, r, u.ID, code)
}
func (h *handlers) passkeyTOTP(w http.ResponseWriter, r *http.Request, id int64, code string) bool {
	secret, enabled, err := h.Store.TOTP(r.Context(), id)
	if err != nil {
		serverErr(w, err)
		return false
	}
	if !enabled {
		return true
	}
	if strings.TrimSpace(code) == "" {
		fail(w, 428, "authenticator code required")
		return false
	}
	if !auth.VerifyTOTPOnce("staff:"+auth.SHA256Hex(secret), secret, code, time.Now()) {
		h.passkeyFailure(w, r)
		return false
	}
	return true
}
func ceremonyBinding(r *http.Request, staffID int64) string {
	if staffID == 0 {
		return ""
	}
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "missing"
	}
	return auth.SHA256Hex(c.Value)
}
func (h *handlers) saveCeremony(w http.ResponseWriter, r *http.Request, kind, name string, id *int64, s *webauthn.SessionData) bool {
	token := auth.Token(32)
	staffID := int64(0)
	if id != nil {
		staffID = *id
	}
	err := h.Store.SavePasskeyChallenge(r.Context(), auth.SHA256Hex(token), kind, ceremonyBinding(r, staffID), name, id, s)
	if err != nil {
		if errors.Is(err, store.ErrPasskeyLimit) {
			fail(w, 429, "too many pending passkey requests")
		} else {
			serverErr(w, err)
		}
		return false
	}
	http.SetCookie(w, &http.Cookie{Name: passkeyCookie, Value: token, Path: "/api/admin/passkeys", HttpOnly: true, Secure: h.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 300})
	return true
}
func (h *handlers) consumeCeremony(w http.ResponseWriter, r *http.Request, kind string, staffID int64) (*webauthn.SessionData, string, bool) {
	c, err := r.Cookie(passkeyCookie)
	if err != nil || len(c.Value) > 128 {
		h.passkeyFailure(w, r)
		return nil, "", false
	}
	session, name, err := h.Store.ConsumePasskeyChallenge(r.Context(), auth.SHA256Hex(c.Value), kind, ceremonyBinding(r, staffID), staffID)
	http.SetCookie(w, &http.Cookie{Name: passkeyCookie, Value: "", Path: "/api/admin/passkeys", HttpOnly: true, Secure: h.Secure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.passkeyFailure(w, r)
		} else {
			serverErr(w, err)
		}
		return nil, "", false
	}
	return session, name, true
}
func (h *handlers) listPasskeys(w http.ResponseWriter, r *http.Request) {
	wa, err := h.webAuthn()
	if err != nil {
		ok(w, map[string]any{"available": false, "items": []store.Passkey{}})
		return
	}
	keys, err := h.Store.Passkeys(r.Context(), userFrom(r).ID, wa.Config.RPID)
	if err != nil {
		serverErr(w, err)
		return
	}
	ok(w, map[string]any{"available": true, "items": keys})
}
func (h *handlers) beginPasskeyRegistration(w http.ResponseWriter, r *http.Request) {
	wa, ready := h.passkeyReady(w, r, true)
	if !ready {
		return
	}
	var in struct{ Name, Password, Code string }
	if !decode(r, &in) || strings.TrimSpace(in.Name) == "" || len(in.Name) > 100 {
		fail(w, 400, "a passkey name of 1–100 bytes is required")
		return
	}
	if !h.passkeyReauth(w, r, in.Password, in.Code) {
		return
	}
	actor := userFrom(r)
	u, err := h.passkeyUser(r.Context(), actor, wa.Config.RPID)
	if err != nil {
		serverErr(w, err)
		return
	}
	if len(u.keys) >= 10 {
		fail(w, 409, "at most 10 passkeys per staff account")
		return
	}
	descriptors := []protocol.CredentialDescriptor{}
	for _, key := range u.keys {
		descriptors = append(descriptors, key.Credential.Descriptor())
	}
	options, session, err := wa.BeginRegistration(u, webauthn.WithExclusions(descriptors), webauthn.WithRegistrationOrigin(wa.Config.RPOrigins[0]))
	if err != nil {
		serverErr(w, err)
		return
	}
	if h.saveCeremony(w, r, "register", in.Name, &actor.ID, session) {
		ok(w, options)
	}
}

type passkeyFinish struct {
	Credential json.RawMessage `json:"credential"`
	Code       string          `json:"code"`
}

func credentialRequest(r *http.Request, b json.RawMessage) *http.Request {
	clone := r.Clone(r.Context())
	clone.Body = io.NopCloser(bytes.NewReader(b))
	clone.ContentLength = int64(len(b))
	return clone
}
func (h *handlers) finishPasskeyRegistration(w http.ResponseWriter, r *http.Request) {
	wa, ready := h.passkeyReady(w, r, false)
	if !ready {
		return
	}
	var in passkeyFinish
	if !decode(r, &in) {
		fail(w, 400, "invalid credential response")
		return
	}
	actor := userFrom(r)
	session, name, valid := h.consumeCeremony(w, r, "register", actor.ID)
	if !valid {
		return
	}
	u, err := h.passkeyUser(r.Context(), actor, wa.Config.RPID)
	if err != nil {
		serverErr(w, err)
		return
	}
	key, err := wa.FinishRegistration(u, *session, credentialRequest(r, in.Credential))
	if err != nil {
		h.passkeyFailure(w, r)
		return
	}
	if err = h.Store.AddPasskey(r.Context(), actor.ID, wa.Config.RPID, name, key); err != nil {
		fail(w, 409, "passkey already registered or limit reached")
		return
	}
	ok(w, map[string]bool{"ok": true})
}
func (h *handlers) beginPasskeyLogin(w http.ResponseWriter, r *http.Request) {
	wa, ready := h.passkeyReady(w, r, true)
	if !ready {
		return
	}
	options, session, err := wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired), webauthn.WithLoginOrigin(wa.Config.RPOrigins[0]))
	if err != nil {
		serverErr(w, err)
		return
	}
	if h.saveCeremony(w, r, "login", "", nil, session) {
		ok(w, options)
	}
}
func (h *handlers) finishPasskeyLogin(w http.ResponseWriter, r *http.Request) {
	wa, ready := h.passkeyReady(w, r, false)
	if !ready {
		return
	}
	var in passkeyFinish
	if !decode(r, &in) {
		fail(w, 400, "invalid credential response")
		return
	}
	session, _, valid := h.consumeCeremony(w, r, "login", 0)
	if !valid {
		return
	}
	lookup := func(rawID, handle []byte) (webauthn.User, error) {
		id, err := h.Store.PasskeyOwner(r.Context(), rawID, handle, wa.Config.RPID)
		if err != nil {
			return nil, err
		}
		u, err := h.Store.StaffByID(r.Context(), id)
		if err != nil {
			return nil, err
		}
		if !u.IsStaff() || u.Status != "active" {
			return nil, store.ErrNotFound
		}
		return h.passkeyUser(r.Context(), u, wa.Config.RPID)
	}
	user, key, err := wa.FinishPasskeyLogin(lookup, *session, credentialRequest(r, in.Credential))
	if err != nil || key.Authenticator.CloneWarning {
		h.passkeyFailure(w, r)
		return
	}
	u := user.(*passkeyUser)
	if !h.passkeyTOTP(w, r, u.principal.ID, in.Code) {
		return
	}
	old := ""
	for _, item := range u.keys {
		if item.ID == base64.RawURLEncoding.EncodeToString(key.ID) {
			old = item.Stored
			break
		}
	}
	if err = h.Store.UpdatePasskey(r.Context(), u.principal.ID, wa.Config.RPID, key, old); err != nil {
		h.passkeyFailure(w, r)
		return
	}
	sess, err := h.Sessions.Create(r.Context(), u.principal.ID, true)
	if err != nil {
		serverErr(w, err)
		return
	}
	if h.Logins != nil {
		h.Logins.Reset(ratelimit.ClientIP(r))
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: sess.ID, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.Secure, Expires: sess.ExpiresAt})
	ok(w, map[string]any{"id": u.principal.ID, "email": u.principal.Email, "role": u.principal.Role})
}
func (h *handlers) deletePasskey(w http.ResponseWriter, r *http.Request) {
	if _, ready := h.passkeyReady(w, r, false); !ready {
		return
	}
	var in struct{ Password, Code string }
	if !decode(r, &in) {
		fail(w, 400, "invalid request")
		return
	}
	if !h.passkeyReauth(w, r, in.Password, in.Code) {
		return
	}
	err := h.Store.DeletePasskey(r.Context(), userFrom(r).ID, r.PathValue("key"))
	if errors.Is(err, store.ErrNotFound) {
		fail(w, 404, "passkey not found")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	ok(w, map[string]bool{"ok": true})
}
