package control

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"rustdesk-control/internal/fakeagent"
)

func TestFakeAgentEndToEndHTTPS(t *testing.T) {
	f := setup(t)
	server := httptest.NewTLSServer(f.h)
	defer server.Close()
	client := server.Client()
	client.Jar, _ = cookiejar.New(nil)
	a, e := fakeagent.New(server.URL, client)
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Enroll(); e != nil {
		t.Fatal(e)
	}
	status, e := a.Heartbeat()
	if e != nil || status != "pending" || a.PasswordVersion != 0 {
		t.Fatal("pending", e)
	}
	res, e := client.Post(server.URL+"/api/v1/admin/login", "application/json", bytes.NewBufferString(`{"password":"test-admin-password"}`))
	if e != nil {
		t.Fatal(e)
	}
	var session map[string]string
	json.NewDecoder(res.Body).Decode(&session)
	res.Body.Close()
	admin := func(path string) {
		t.Helper()
		r, _ := http.NewRequest("POST", server.URL+path, nil)
		r.Header.Set("X-CSRF-Token", session["csrf_token"])
		res, e := client.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("admin HTTP %d", res.StatusCode)
		}
	}
	admin("/api/v1/admin/devices/1/approve")
	status, e = a.Heartbeat()
	if e != nil || status != "approved" || a.PasswordVersion != 1 {
		t.Fatal("approved", e)
	}
	if _, e = a.Heartbeat(); e != nil {
		t.Fatal(e)
	}
	admin("/api/v1/admin/devices/1/rotate-password")
	if _, e = a.Heartbeat(); e != nil || a.PasswordVersion != 2 {
		t.Fatal("rotation", e)
	}
	if _, e = a.Heartbeat(); e != nil {
		t.Fatal(e)
	}
	var applied int
	if e = f.s.DB.QueryRow("SELECT applied_password_version FROM devices WHERE id=1").Scan(&applied); e != nil || applied != 2 {
		t.Fatal("ack", e)
	}
}
