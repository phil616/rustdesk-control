package control

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	s      *Server
	h      http.Handler
	cookie *http.Cookie
	csrf   string
	pub    ed25519.PublicKey
	priv   ed25519.PrivateKey
	uuid   string
	t      *testing.T
}

func setup(t *testing.T) *fixture {
	t.Helper()
	s, e := Open(filepath.Join(t.TempDir(), "control.db"), bytes.Repeat([]byte{42}, 32), "test-admin-password")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.DB.Close() })
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	return &fixture{s: s, h: s.Handler(nil), pub: pub, priv: priv, uuid: "a8098c1a-f86e-41da-bd1a-34068569ad08", t: t}
}
func (f *fixture) request(method, path string, b any, admin bool) *httptest.ResponseRecorder {
	f.t.Helper()
	raw, _ := json.Marshal(b)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.RemoteAddr = "127.0.0.1:1000"
	if admin && f.cookie != nil {
		r.AddCookie(f.cookie)
		r.Header.Set("X-CSRF-Token", f.csrf)
	}
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	return w
}
func expect(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d want %d: %s", w.Code, status, w.Body.String())
	}
}
func (f *fixture) login() {
	w := f.request("POST", "/api/v1/admin/login", map[string]string{"password": "test-admin-password"}, false)
	expect(f.t, w, 200)
	f.cookie = w.Result().Cookies()[0]
	var b map[string]string
	json.Unmarshal(w.Body.Bytes(), &b)
	f.csrf = b["csrf_token"]
}
func (f *fixture) signed(path string, body any, ts int64, nonce string) *http.Request {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", path, bytes.NewReader(raw))
	r.RemoteAddr = "127.0.0.1:1000"
	stamp := strconv.FormatInt(ts, 10)
	r.Header.Set("X-RDC-Device-ID", f.uuid)
	r.Header.Set("X-RDC-Timestamp", stamp)
	r.Header.Set("X-RDC-Nonce", nonce)
	r.Header.Set("X-RDC-Signature", base64.StdEncoding.EncodeToString(ed25519.Sign(f.priv, Canonical("POST", path, stamp, nonce, raw))))
	return r
}
func (f *fixture) send(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	return w
}
func (f *fixture) enroll() *httptest.ResponseRecorder {
	ts := f.s.now().Unix()
	b := Enrollment{f.uuid, base64.StdEncoding.EncodeToString(f.pub), "123456789", "test-host", "linux", "Debian", "x86_64", "1.4.9", "1.0.0", ts}
	return f.send(f.signed("/api/v1/agent/enroll", b, ts, randomToken(24)))
}
func (f *fixture) heartbeat(pv, pw int64) *httptest.ResponseRecorder {
	return f.send(f.signed("/api/v1/agent/heartbeat", Heartbeat{"123456789", "test-host", "1.4.9", "1.0.0", pv, pw}, f.s.now().Unix(), randomToken(24)))
}
func TestAuthenticationAndSessions(t *testing.T) {
	f := setup(t)
	expect(t, f.request("GET", "/api/v1/admin/devices", nil, false), 401)
	expect(t, f.request("POST", "/api/v1/admin/login", map[string]string{"password": "bad"}, false), 401)
	f.login()
	if !f.cookie.HttpOnly || !f.cookie.Secure || f.cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe cookie")
	}
	expect(t, f.request("GET", "/api/v1/admin/session", nil, true), 200)
	csrf := f.csrf
	f.csrf = "bad"
	expect(t, f.request("POST", "/api/v1/admin/logout", nil, true), 403)
	f.csrf = csrf
	expect(t, f.request("POST", "/api/v1/admin/logout", nil, true), 200)
	expect(t, f.request("GET", "/api/v1/admin/session", nil, true), 401)
	f.login()
	f.s.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	expect(t, f.request("GET", "/api/v1/admin/session", nil, true), 401)
}
func TestLifecycle(t *testing.T) {
	f := setup(t)
	f.login()
	expect(t, f.enroll(), 200)
	w := f.heartbeat(0, 0)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), "password") {
		t.Fatal("pending got credential")
	}
	expect(t, f.request("POST", "/api/v1/admin/devices/1/approve", nil, true), 200)
	w = f.heartbeat(0, 0)
	expect(t, w, 200)
	var p Policy
	json.Unmarshal(w.Body.Bytes(), &p)
	if p.ManagedAccess == nil || p.ManagedAccess.PasswordVersion != 1 || len(p.ManagedAccess.Password) < 24 {
		t.Fatal("no managed password")
	}
	old := p.ManagedAccess.Password
	var cipher []byte
	f.s.DB.QueryRow("SELECT ciphertext FROM device_credentials WHERE device_id=1").Scan(&cipher)
	if bytes.Contains(cipher, []byte(old)) {
		t.Fatal("plaintext stored")
	}
	w = f.request("GET", "/api/v1/admin/devices/1/credential", nil, true)
	expect(t, w, 200)
	if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), old) {
		t.Fatal("credential response")
	}
	expect(t, f.heartbeat(p.PolicyVersion, 1), 200)
	w = f.request("GET", "/api/v1/admin/devices/1", nil, true)
	var device Device
	json.Unmarshal(w.Body.Bytes(), &device)
	if !device.Online || !device.PasswordSynced {
		t.Fatal("sync status")
	}
	expect(t, f.request("POST", "/api/v1/admin/devices/1/rotate-password", nil, true), 200)
	w = f.heartbeat(1, 1)
	expect(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &p)
	if p.ManagedAccess.PasswordVersion != 2 || p.ManagedAccess.Password == old {
		t.Fatal("rotation")
	}
	settings := RustDesk{"rd.example.com", "relay.example.com", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)), ""}
	w = f.request("PUT", "/api/v1/admin/settings/rustdesk", settings, true)
	expect(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &p)
	if p.PolicyVersion != 2 {
		t.Fatal("policy version")
	}
	w = f.request("PUT", "/api/v1/admin/settings/rustdesk", settings, true)
	json.Unmarshal(w.Body.Bytes(), &p)
	if p.PolicyVersion != 2 {
		t.Fatal("unchanged settings bumped policy")
	}
	expect(t, f.heartbeat(2, 2), 200)
	w = f.request("GET", "/api/v1/admin/audit", nil, true)
	if strings.Contains(w.Body.String(), old) || !strings.Contains(w.Body.String(), "credential revealed") {
		t.Fatal("audit")
	}
	expect(t, f.request("POST", "/api/v1/admin/devices/1/reject", nil, true), 200)
	w = f.heartbeat(2, 2)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), "password") {
		t.Fatal("rejected credential leak")
	}
	expect(t, f.request("GET", "/api/v1/admin/devices/1/credential", nil, true), 404)
	expect(t, f.request("POST", "/api/v1/admin/devices/1/reset-identity", nil, true), 200)
	expect(t, f.heartbeat(0, 0), 401)
	expect(t, f.request("POST", "/api/v1/admin/devices/1/approve", nil, true), 409)
	f.pub, f.priv, _ = ed25519.GenerateKey(rand.Reader)
	expect(t, f.enroll(), 200)
	expect(t, f.request("POST", "/api/v1/admin/devices/1/approve", nil, true), 200)
	expect(t, f.request("DELETE", "/api/v1/admin/devices/1", nil, true), 200)
	expect(t, f.heartbeat(0, 0), 401)
}
func TestSignatureAndIdentity(t *testing.T) {
	f := setup(t)
	expect(t, f.enroll(), 200)
	body := Heartbeat{"123", "test", "1.4.9", "1.0.0", 0, 0}
	for _, offset := range []int64{-301, 301} {
		expect(t, f.send(f.signed("/api/v1/agent/heartbeat", body, f.s.now().Unix()+offset, randomToken(24))), 401)
	}
	r := f.signed("/api/v1/agent/heartbeat", body, f.s.now().Unix(), randomToken(24))
	r.Header.Set("X-RDC-Signature", base64.StdEncoding.EncodeToString(make([]byte, 64)))
	expect(t, f.send(r), 401)
	nonce := randomToken(24)
	ts := f.s.now().Unix()
	expect(t, f.send(f.signed("/api/v1/agent/heartbeat", body, ts, nonce)), 200)
	expect(t, f.send(f.signed("/api/v1/agent/heartbeat", body, ts, nonce)), 409)
	f.pub, f.priv, _ = ed25519.GenerateKey(rand.Reader)
	expect(t, f.enroll(), 409)
	f.uuid = "not-a-uuid"
	expect(t, f.enroll(), 400)
}
func TestOnlineBoundaryAndFilters(t *testing.T) {
	f := setup(t)
	f.login()
	expect(t, f.enroll(), 200)
	now := f.s.now()
	for _, seconds := range []int{120, 121} {
		f.s.now = func() time.Time { return now.Add(time.Duration(seconds) * time.Second) }
		w := f.request("GET", "/api/v1/admin/devices/1", nil, true)
		var d Device
		json.Unmarshal(w.Body.Bytes(), &d)
		if d.Online != (seconds <= 120) {
			t.Fatal("online boundary")
		}
	}
	for _, q := range []string{"search=test-host", "status=pending&online=false", "page=1&page_size=1"} {
		w := f.request("GET", "/api/v1/admin/devices?"+q, nil, true)
		expect(t, w, 200)
		if !strings.Contains(w.Body.String(), "test-host") {
			t.Fatal("filter")
		}
	}
	expect(t, f.request("GET", "/api/v1/admin/dashboard", nil, true), 200)
}
func TestEncryption(t *testing.T) {
	a, e := newCipher(bytes.Repeat([]byte{7}, 32))
	if e != nil {
		t.Fatal(e)
	}
	n, c, e := encrypt(a, "test-only-secret", 7)
	if e != nil {
		t.Fatal(e)
	}
	p, e := a.Open(nil, n, c, []byte("7"))
	if e != nil || string(p) != "test-only-secret" {
		t.Fatal("round trip")
	}
	if _, e = a.Open(nil, n, c, []byte("8")); e == nil {
		t.Fatal("device binding")
	}
	c[0] ^= 1
	if _, e = a.Open(nil, n, c, []byte("7")); e == nil {
		t.Fatal("tampering")
	}
	if _, e = newCipher(nil); e == nil {
		t.Fatal("missing master key")
	}
}
func TestCSRFAndPasswordChange(t *testing.T) {
	f := setup(t)
	f.login()
	r := httptest.NewRequest("POST", "/api/v1/admin/logout", nil)
	r.AddCookie(f.cookie)
	r.Header.Set("X-CSRF-Token", f.csrf)
	r.Header.Set("Origin", "https://evil.example")
	expect(t, f.send(r), 403)
	expect(t, f.request("PUT", "/api/v1/admin/password", map[string]string{"current_password": "test-admin-password", "new_password": "new-test-admin-password"}, true), 200)
	expect(t, f.request("GET", "/api/v1/admin/session", nil, true), 401)
	expect(t, f.request("POST", "/api/v1/admin/login", map[string]string{"password": "new-test-admin-password"}, false), 200)
}
func TestBootstrapRateLimit(t *testing.T) {
	f := setup(t)
	for i := 0; i < 60; i++ {
		expect(t, f.request("GET", "/api/v1/agent/bootstrap", nil, false), 200)
	}
	expect(t, f.request("GET", "/api/v1/agent/bootstrap", nil, false), 429)
}
func TestCanonicalCrossLanguage(t *testing.T) {
	want := "POST\n/api/v1/agent/heartbeat\n1\nnonce\n44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"
	if string(Canonical("POST", "/api/v1/agent/heartbeat", "1", "nonce", []byte("{}"))) != want {
		t.Fatal("canonical mismatch")
	}
}

func TestTrustedProxy(t *testing.T) {
	f := setup(t)
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.50")
	if f.s.clientIP(r) != "192.0.2.1" {
		t.Fatal("untrusted header accepted")
	}
	_, network, _ := net.ParseCIDR("127.0.0.1/32")
	f.s.TrustedProxies = []*net.IPNet{network}
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.50, 198.51.100.2")
	if f.s.clientIP(r) != "198.51.100.2" {
		t.Fatal("spoofed leftmost IP trusted")
	}
}
