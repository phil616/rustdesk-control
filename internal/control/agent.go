package control

import (
	"bytes"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

type RustDesk struct {
	IDServer    string `json:"id_server"`
	RelayServer string `json:"relay_server"`
	Key         string `json:"key"`
	APIServer   string `json:"api_server"`
}
type Policy struct {
	ProtocolVersion int      `json:"protocol_version"`
	ServerTime      int64    `json:"server_time"`
	DeviceStatus    string   `json:"device_status,omitempty"`
	PolicyVersion   int64    `json:"policy_version"`
	RustDesk        RustDesk `json:"rustdesk"`
	ManagedAccess   *Access  `json:"managed_access,omitempty"`
}
type Access struct {
	Enabled         bool   `json:"enabled"`
	Password        string `json:"password"`
	PasswordVersion int64  `json:"password_version"`
	LockPassword    bool   `json:"lock_password"`
}
type queryer interface{ QueryRow(string, ...any) *sql.Row }

func (s *Server) policy(q queryer) (Policy, error) {
	p := Policy{ProtocolVersion: 1, ServerTime: s.now().Unix()}
	e := q.QueryRow("SELECT id_server,relay_server,public_key,policy_version FROM settings WHERE id=1").Scan(&p.RustDesk.IDServer, &p.RustDesk.RelayServer, &p.RustDesk.Key, &p.PolicyVersion)
	return p, e
}
func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	if !s.rate(r, "bootstrap", 60) {
		bad(w, 429, "try again later")
		return
	}
	p, e := s.policy(s.DB)
	if e != nil {
		internalError(w, e)
		return
	}
	reply(w, 200, p)
}

type Enrollment struct {
	DeviceUUID           string `json:"device_uuid"`
	PublicKey            string `json:"public_key"`
	RustDeskID           string `json:"rustdesk_id"`
	Hostname             string `json:"hostname"`
	OS                   string `json:"os"`
	OSVersion            string `json:"os_version"`
	Arch                 string `json:"arch"`
	RustDeskVersion      string `json:"rustdesk_version"`
	ManagedClientVersion string `json:"managed_client_version"`
	Timestamp            int64  `json:"timestamp"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func bounded(values ...string) bool {
	for _, v := range values {
		if len(v) > 255 {
			return false
		}
	}
	return true
}
func rawBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
	if e != nil {
		bad(w, 400, "body too large")
		return nil, false
	}
	return b, true
}
func (s *Server) verify(r *http.Request, key []byte, body []byte) bool {
	return r.URL.RawQuery == "" && validSignature(key, r.Method, r.URL.EscapedPath(), r.Header.Get("X-RDC-Timestamp"), r.Header.Get("X-RDC-Nonce"), r.Header.Get("X-RDC-Signature"), body, s.now())
}
func (s *Server) nonce(tx *sql.Tx, id int64, r *http.Request) error {
	_, e := tx.Exec("DELETE FROM agent_nonces WHERE expires_at<?", s.now().Unix())
	if e != nil {
		return e
	}
	ts, _ := strconv.ParseInt(r.Header.Get("X-RDC-Timestamp"), 10, 64)
	_, e = tx.Exec("INSERT INTO agent_nonces VALUES(?,?,?)", id, r.Header.Get("X-RDC-Nonce"), ts+301)
	return e
}
func (s *Server) enroll(w http.ResponseWriter, r *http.Request) {
	if !s.rate(r, "enroll", 30) {
		bad(w, 429, "try again later")
		return
	}
	raw, ok := rawBody(w, r)
	if !ok {
		return
	}
	var b Enrollment
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&b) != nil || d.Decode(new(any)) != io.EOF || !uuidPattern.MatchString(b.DeviceUUID) || b.Hostname == "" || !bounded(b.RustDeskID, b.Hostname, b.OS, b.OSVersion, b.Arch, b.RustDeskVersion, b.ManagedClientVersion) {
		bad(w, 400, "invalid enrollment")
		return
	}
	key, e := base64.StdEncoding.DecodeString(b.PublicKey)
	if e != nil || len(key) != ed25519.PublicKeySize || r.Header.Get("X-RDC-Device-ID") != b.DeviceUUID || r.Header.Get("X-RDC-Timestamp") != strconv.FormatInt(b.Timestamp, 10) || !s.verify(r, key, raw) {
		bad(w, 401, "invalid signature")
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		internalError(w, e)
		return
	}
	defer tx.Rollback()
	var id int64
	var old []byte
	e = tx.QueryRow("SELECT d.id,a.ed25519_public_key FROM devices d LEFT JOIN device_auth a ON a.device_id=d.id WHERE device_uuid=?", b.DeviceUUID).Scan(&id, &old)
	now := s.now().Unix()
	if e == sql.ErrNoRows {
		res, err := tx.Exec("INSERT INTO devices(device_uuid,rustdesk_id,hostname,os,os_version,arch,rustdesk_version,managed_client_version,first_seen_at,last_seen_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", b.DeviceUUID, b.RustDeskID, b.Hostname, b.OS, b.OSVersion, b.Arch, b.RustDeskVersion, b.ManagedClientVersion, now, now, now, now)
		if err != nil {
			internalError(w, err)
			return
		}
		id, e = res.LastInsertId()
	} else if e == nil && old != nil && !bytes.Equal(old, key) {
		bad(w, 409, "identity key conflict; administrator reset required")
		return
	}
	if e != nil {
		internalError(w, e)
		return
	}
	if _, e = tx.Exec("INSERT INTO device_auth VALUES(?,?,?,?) ON CONFLICT(device_id) DO NOTHING", id, key, now, now); e != nil {
		internalError(w, e)
		return
	}
	if e = s.nonce(tx, id, r); e != nil {
		bad(w, 409, "replayed request")
		return
	}
	var status string
	if e = tx.QueryRow("SELECT status FROM devices WHERE id=?", id).Scan(&status); e != nil {
		internalError(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		internalError(w, e)
		return
	}
	reply(w, 200, map[string]any{"id": id, "device_status": status})
}

type Heartbeat struct {
	RustDeskID           string `json:"rustdesk_id"`
	Hostname             string `json:"hostname"`
	RustDeskVersion      string `json:"rustdesk_version"`
	ManagedClientVersion string `json:"managed_client_version"`
	PolicyVersion        int64  `json:"policy_version"`
	PasswordVersion      int64  `json:"password_version"`
}

func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request) {
	raw, ok := rawBody(w, r)
	if !ok {
		return
	}
	var b Heartbeat
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&b) != nil || d.Decode(new(any)) != io.EOF || b.PolicyVersion < 0 || b.PasswordVersion < 0 || !bounded(b.RustDeskID, b.Hostname, b.RustDeskVersion, b.ManagedClientVersion) {
		bad(w, 400, "invalid heartbeat")
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		internalError(w, e)
		return
	}
	defer tx.Rollback()
	var id int64
	var key []byte
	var status string
	e = tx.QueryRow("SELECT d.id,a.ed25519_public_key,d.status FROM devices d JOIN device_auth a ON a.device_id=d.id WHERE device_uuid=?", r.Header.Get("X-RDC-Device-ID")).Scan(&id, &key, &status)
	if e != nil || !s.verify(r, key, raw) {
		bad(w, 401, "invalid signature")
		return
	}
	if e = s.nonce(tx, id, r); e != nil {
		bad(w, 409, "replayed request")
		return
	}
	p, e := s.policy(tx)
	if e != nil {
		internalError(w, e)
		return
	}
	p.DeviceStatus = status
	var pv int64
	if e = tx.QueryRow("SELECT COALESCE((SELECT password_version FROM device_credentials WHERE device_id=?),0)", id).Scan(&pv); e != nil {
		internalError(w, e)
		return
	}
	if b.PolicyVersion > p.PolicyVersion || b.PasswordVersion > pv {
		bad(w, 400, "reported version exceeds server version")
		return
	}
	if status == "approved" {
		var n, c []byte
		var version int64
		e = tx.QueryRow("SELECT nonce,ciphertext,password_version FROM device_credentials WHERE device_id=?", id).Scan(&n, &c, &version)
		if e != nil {
			internalError(w, e)
			return
		}
		if len(n) != s.cipher.NonceSize() {
			bad(w, 500, "credential unavailable")
			return
		}
		password, e := s.cipher.Open(nil, n, c, []byte(strconv.FormatInt(id, 10)))
		if e != nil {
			bad(w, 500, "credential unavailable")
			return
		}
		p.ManagedAccess = &Access{true, string(password), version, true}
	}
	now := s.now().Unix()
	_, e = tx.Exec("UPDATE devices SET rustdesk_id=?,hostname=?,rustdesk_version=?,managed_client_version=?,last_seen_at=?,updated_at=?,applied_policy_version=?,applied_password_version=? WHERE id=?", b.RustDeskID, b.Hostname, b.RustDeskVersion, b.ManagedClientVersion, now, now, b.PolicyVersion, b.PasswordVersion, id)
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		internalError(w, e)
		return
	}
	reply(w, 200, p)
}
