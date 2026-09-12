// Package fakeagent implements the public wire protocol without a RustDesk GUI.
package fakeagent

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Agent struct {
	UUID                           string
	Public                         ed25519.PublicKey
	Private                        ed25519.PrivateKey
	Base                           string
	Client                         *http.Client
	PolicyVersion, PasswordVersion int64
}

func New(base string, client *http.Client) (*Agent, error) {
	pub, priv, e := ed25519.GenerateKey(rand.Reader)
	return &Agent{UUID: uuid.NewString(), Public: pub, Private: priv, Base: strings.TrimRight(base, "/"), Client: client}, e
}
func (a *Agent) post(path string, body any, ts int64) (map[string]any, error) {
	raw, e := json.Marshal(body)
	if e != nil {
		return nil, e
	}
	stamp := strconv.FormatInt(ts, 10)
	nonce := uuid.NewString()
	hash := sha256.Sum256(raw)
	canonical := strings.Join([]string{"POST", path, stamp, nonce, hex.EncodeToString(hash[:])}, "\n")
	r, e := http.NewRequest("POST", a.Base+path, bytes.NewReader(raw))
	if e != nil {
		return nil, e
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-RDC-Device-ID", a.UUID)
	r.Header.Set("X-RDC-Timestamp", stamp)
	r.Header.Set("X-RDC-Nonce", nonce)
	r.Header.Set("X-RDC-Signature", base64.StdEncoding.EncodeToString(ed25519.Sign(a.Private, []byte(canonical))))
	res, e := a.Client.Do(r)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("agent request failed: HTTP %d", res.StatusCode)
	}
	var result map[string]any
	e = json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&result)
	return result, e
}
func (a *Agent) Enroll() error {
	ts := time.Now().Unix()
	_, e := a.post("/api/v1/agent/enroll", map[string]any{"device_uuid": a.UUID, "public_key": base64.StdEncoding.EncodeToString(a.Public), "rustdesk_id": "999000111", "hostname": "fake-agent", "os": "linux", "os_version": "test", "arch": "x86_64", "rustdesk_version": "1.4.9", "managed_client_version": "1.0.0", "timestamp": ts}, ts)
	return e
}
func (a *Agent) Heartbeat() (string, error) {
	r, e := a.post("/api/v1/agent/heartbeat", map[string]any{"rustdesk_id": "999000111", "hostname": "fake-agent", "rustdesk_version": "1.4.9", "managed_client_version": "1.0.0", "policy_version": a.PolicyVersion, "password_version": a.PasswordVersion}, time.Now().Unix())
	if e != nil {
		return "", e
	}
	if p, ok := r["policy_version"].(float64); ok {
		a.PolicyVersion = int64(p)
	}
	if access, ok := r["managed_access"].(map[string]any); ok {
		if v, ok := access["password_version"].(float64); ok {
			a.PasswordVersion = int64(v)
		}
	}
	status, _ := r["device_status"].(string)
	return status, nil
}
