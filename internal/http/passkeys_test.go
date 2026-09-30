package http

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/zeptop-dev/captain/internal/auth"
)

type testPasskey struct {
	key        *ecdsa.PrivateKey
	id, handle []byte
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
func decode64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func ceremonyCookies(c *client, head http.Header) map[string]string {
	cookies := ""
	if c.cookie != nil {
		cookies = c.cookie.Name + "=" + c.cookie.Value + "; "
	}
	for _, cookie := range (&http.Response{Header: head}).Cookies() {
		if cookie.Name == "captain_passkey" {
			cookies += cookie.Name + "=" + cookie.Value
		}
	}
	return map[string]string{"Cookie": cookies}
}
func enrollPasskey(t *testing.T, r *rig, beforeFinish ...func(http.Header, map[string]any)) testPasskey {
	t.Helper()
	code, b, head := r.c.do("POST", "/api/admin/passkeys/register/begin", map[string]string{"Name": "test device", "Password": "password123"}, nil)
	if code != 200 {
		t.Fatalf("begin %d %s", code, b)
	}
	var options struct {
		PublicKey struct {
			Challenge string
			User      struct{ ID string }
		}
	}
	options = mustJSON[struct {
		PublicKey struct {
			Challenge string
			User      struct{ ID string }
		}
	}](t, b)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pk := testPasskey{key: key, id: make([]byte, 32), handle: decode64(t, options.PublicKey.User.ID)}
	if _, err = rand.Read(pk.id); err != nil {
		t.Fatal(err)
	}
	cose, err := cbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: key.X.FillBytes(make([]byte, 32)), -3: key.Y.FillBytes(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	rp := sha256.Sum256([]byte("localhost"))
	data := append(rp[:], 0x45, 0, 0, 0, 0)
	data = append(data, make([]byte, 16)...)
	data = binary.BigEndian.AppendUint16(data, uint16(len(pk.id)))
	data = append(data, pk.id...)
	data = append(data, cose...)
	attestation, err := cbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": data})
	if err != nil {
		t.Fatal(err)
	}
	client, _ := json.Marshal(map[string]any{"type": "webauthn.create", "challenge": options.PublicKey.Challenge, "origin": "http://localhost", "crossOrigin": false})
	credential := map[string]any{"id": b64(pk.id), "rawId": b64(pk.id), "type": "public-key", "response": map[string]any{"clientDataJSON": b64(client), "attestationObject": b64(attestation), "transports": []string{"internal"}}}
	for _, hook := range beforeFinish {
		hook(head, credential)
	}
	code, b, _ = r.c.do("POST", "/api/admin/passkeys/register/finish", map[string]any{"credential": credential}, ceremonyCookies(r.c, head))
	if code != 200 {
		t.Fatalf("finish enrollment %d %s", code, b)
	}
	if code, _, _ := r.c.do("POST", "/api/admin/passkeys/register/finish", map[string]any{"credential": credential}, ceremonyCookies(r.c, head)); code != 401 {
		t.Fatal("registration challenge replay", code)
	}
	return pk
}
func assertion(t *testing.T, c *client, pk testPasskey, origin, rp string, flags byte, counter uint32) (map[string]any, map[string]string) {
	t.Helper()
	code, b, head := c.do("POST", "/api/admin/passkeys/login/begin", nil, nil)
	if code != 200 {
		t.Fatal(code, string(b))
	}
	options := mustJSON[struct{ PublicKey struct{ Challenge string } }](t, b)
	data, _ := json.Marshal(map[string]any{"type": "webauthn.get", "challenge": options.PublicKey.Challenge, "origin": origin, "crossOrigin": false})
	rpHash := sha256.Sum256([]byte(rp))
	authData := append(rpHash[:], flags)
	authData = binary.BigEndian.AppendUint32(authData, counter)
	clientHash := sha256.Sum256(data)
	signed := sha256.Sum256(append(append([]byte{}, authData...), clientHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, pk.key, signed[:])
	if err != nil {
		t.Fatal(err)
	}
	credential := map[string]any{"id": b64(pk.id), "rawId": b64(pk.id), "type": "public-key", "response": map[string]any{"clientDataJSON": b64(data), "authenticatorData": b64(authData), "signature": b64(sig), "userHandle": b64(pk.handle)}}
	return map[string]any{"credential": credential}, ceremonyCookies(c, head)
}
func TestPasskeyLoginRenumberRevokeAndReplay(t *testing.T) {
	r := newRigAt(t, "http://localhost")
	pk := enrollPasskey(t, r)
	if code, b, _ := r.c.do("PUT", "/api/admin/admins/1/id", map[string]int64{"id": 42}, nil); code != 200 {
		t.Fatal(code, string(b))
	}
	c := &client{t: t, srv: r.srv}
	body, headers := assertion(t, c, pk, "http://localhost", "localhost", 5, 1)
	code, b, _ := c.do("POST", "/api/admin/passkeys/login/finish", body, headers)
	if code != 200 || mustJSON[map[string]any](t, b)["id"] != float64(42) {
		t.Fatalf("login %d %s", code, b)
	}
	if code, _, _ := c.do("GET", "/api/portal/me", nil, nil); code == 200 {
		t.Fatal("staff entered portal")
	}
	if code, _, _ := c.do("POST", "/api/admin/passkeys/login/finish", body, headers); code != 401 {
		t.Fatal("login replay", code)
	}
	// Clone counter rollback cannot authenticate.
	body, headers = assertion(t, c, pk, "http://localhost", "localhost", 5, 1)
	if code, _, _ := c.do("POST", "/api/admin/passkeys/login/finish", body, headers); code != 401 {
		t.Fatal("reused signature counter", code)
	}
	if code, b, _ := r.c.do("DELETE", "/api/admin/passkeys/"+b64(pk.id), map[string]string{"Password": "password123"}, nil); code != 200 {
		t.Fatal(code, string(b))
	}
	body, headers = assertion(t, c, pk, "http://localhost", "localhost", 5, 2)
	if code, _, _ := c.do("POST", "/api/admin/passkeys/login/finish", body, headers); code != 401 {
		t.Fatal("revoked credential", code)
	}
}
func TestPasskeyOriginVerificationAndSessionBinding(t *testing.T) {
	for _, tt := range []struct {
		name, origin, rp string
		flags            byte
	}{{"origin", "https://evil.example.com", "localhost", 5}, {"rp", "http://localhost", "evil.example.com", 5}, {"uv", "http://localhost", "localhost", 1}} {
		t.Run(tt.name, func(t *testing.T) {
			r := newRigAt(t, "http://localhost")
			pk := enrollPasskey(t, r)
			c := &client{t: t, srv: r.srv}
			body, headers := assertion(t, c, pk, tt.origin, tt.rp, tt.flags, 1)
			if code, _, _ := c.do("POST", "/api/admin/passkeys/login/finish", body, headers); code != 401 {
				t.Fatal("invalid assertion accepted", code)
			}
		})
	}
	r := newRigAt(t, "http://localhost")
	if code, _, _ := r.c.do("POST", "/api/admin/passkeys/register/begin", map[string]string{"Name": "stolen session", "Password": "wrong"}, nil); code != 401 {
		t.Fatal("enrollment without reauth", code)
	}
	if code, _, _ := r.c.do("POST", "/api/admin/passkeys/login/begin", nil, map[string]string{"Origin": "http://localhost:9999"}); code != 403 {
		t.Fatal("wrong origin port", code)
	}
	token, _, err := r.st.CreateAPIToken(context.Background(), 1, "automation", "full", nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &client{t: t, srv: r.srv, token: token}
	if code, _, _ := c.do("GET", "/api/admin/passkeys", nil, nil); code != 403 {
		t.Fatal("API token accessed passkeys", code)
	}
}
func TestPasskeyTOTPAndExpiryRemainRequired(t *testing.T) {
	r := newRigAt(t, "http://localhost")
	pk := enrollPasskey(t, r)
	secret := auth.NewTOTPSecret()
	if err := r.st.SetTOTP(context.Background(), 1, secret, true); err != nil {
		t.Fatal(err)
	}
	c := &client{t: t, srv: r.srv}
	body, headers := assertion(t, c, pk, "http://localhost", "localhost", 5, 1)
	if code, _, _ := c.do("POST", "/api/admin/passkeys/login/finish", body, headers); code != 428 {
		t.Fatal("passkey bypassed TOTP", code)
	}
	body, headers = assertion(t, c, pk, "http://localhost", "localhost", 5, 2)
	body["code"] = auth.TOTPCode(secret, time.Now())
	if code, b, _ := c.do("POST", "/api/admin/passkeys/login/finish", body, headers); code != 200 {
		t.Fatal(code, string(b))
	}
	body, headers = assertion(t, c, pk, "http://localhost", "localhost", 5, 3)
	if _, err := r.st.DB().Exec(`UPDATE passkey_challenges SET expires_at=1`); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := c.do("POST", "/api/admin/passkeys/login/finish", body, headers); code != 401 {
		t.Fatal("expired challenge", code)
	}
}

func TestPasskeyRegistrationBoundToOriginalStaffSession(t *testing.T) {
	r := newRigAt(t, "http://localhost")
	other := &client{t: t, srv: r.srv}
	if code, _, _ := other.do("POST", "/api/admin/login", map[string]string{"Email": "admin@test", "Password": "password123"}, nil); code != 200 {
		t.Fatal(code)
	}
	enrollPasskey(t, r, func(head http.Header, credential map[string]any) {
		if code, _, _ := other.do("POST", "/api/admin/passkeys/register/finish", map[string]any{"credential": credential}, ceremonyCookies(other, head)); code != 401 {
			t.Fatal("another session used registration challenge", code)
		}
	})
}
