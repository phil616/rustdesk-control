package control

import (
	"database/sql"
	"encoding/base64"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Device struct {
	ID              int64         `json:"id"`
	UUID            string        `json:"device_uuid"`
	RustDeskID      string        `json:"rustdesk_id"`
	Hostname        string        `json:"hostname"`
	OS              string        `json:"os"`
	OSVersion       string        `json:"os_version"`
	Arch            string        `json:"arch"`
	RustDeskVersion string        `json:"rustdesk_version"`
	ManagedVersion  string        `json:"managed_client_version"`
	Status          string        `json:"status"`
	FirstSeen       int64         `json:"first_seen_at"`
	LastSeen        int64         `json:"last_seen_at"`
	ApprovedAt      sql.NullInt64 `json:"-"`
	PolicyVersion   int64         `json:"applied_policy_version"`
	AppliedPassword int64         `json:"applied_password_version"`
	PasswordVersion int64         `json:"password_version"`
	Online          bool          `json:"online"`
	PasswordSynced  bool          `json:"password_synced"`
}

const deviceSelect = `SELECT d.id,d.device_uuid,d.rustdesk_id,d.hostname,d.os,d.os_version,d.arch,d.rustdesk_version,d.managed_client_version,d.status,d.first_seen_at,d.last_seen_at,d.approved_at,d.applied_policy_version,d.applied_password_version,COALESCE(c.password_version,0) FROM devices d LEFT JOIN device_credentials c ON c.device_id=d.id `

type scanner interface{ Scan(...any) error }

func (s *Server) scanDevice(row scanner) (Device, error) {
	var d Device
	e := row.Scan(&d.ID, &d.UUID, &d.RustDeskID, &d.Hostname, &d.OS, &d.OSVersion, &d.Arch, &d.RustDeskVersion, &d.ManagedVersion, &d.Status, &d.FirstSeen, &d.LastSeen, &d.ApprovedAt, &d.PolicyVersion, &d.AppliedPassword, &d.PasswordVersion)
	d.Online = s.now().Unix()-d.LastSeen <= 120
	d.PasswordSynced = d.Status == "approved" && d.PasswordVersion > 0 && d.AppliedPassword == d.PasswordVersion
	return d, e
}
func page(r *http.Request) (int, int) {
	p, _ := strconv.Atoi(r.URL.Query().Get("page"))
	n, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if p < 1 {
		p = 1
	}
	if p > 1000000 {
		p = 1000000
	}
	if n < 1 || n > 100 {
		n = 20
	}
	return p, n
}
func (s *Server) devices(w http.ResponseWriter, r *http.Request) {
	p, n := page(r)
	where := " WHERE 1=1"
	args := []any{}
	if q := r.URL.Query().Get("search"); q != "" {
		where += " AND (d.rustdesk_id LIKE ? ESCAPE '\\' OR d.hostname LIKE ? ESCAPE '\\')"
		q = strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q)
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	if st := r.URL.Query().Get("status"); st != "" {
		where += " AND d.status=?"
		args = append(args, st)
	}
	switch r.URL.Query().Get("online") {
	case "true":
		where += " AND d.last_seen_at>=?"
		args = append(args, s.now().Unix()-120)
	case "false":
		where += " AND d.last_seen_at<?"
		args = append(args, s.now().Unix()-120)
	}
	var total int
	if e := s.DB.QueryRow("SELECT count(*) FROM devices d"+where, args...).Scan(&total); e != nil {
		internalError(w, e)
		return
	}
	rows, e := s.DB.Query(deviceSelect+where+" ORDER BY d.last_seen_at DESC,d.id DESC LIMIT ? OFFSET ?", append(args, n, (p-1)*n)...)
	if e != nil {
		internalError(w, e)
		return
	}
	defer rows.Close()
	items := []Device{}
	for rows.Next() {
		d, e := s.scanDevice(rows)
		if e != nil {
			internalError(w, e)
			return
		}
		items = append(items, d)
	}
	if e = rows.Err(); e != nil {
		internalError(w, e)
		return
	}
	reply(w, 200, map[string]any{"items": items, "total": total, "page": p, "page_size": n})
}
func (s *Server) detail(w http.ResponseWriter, r *http.Request) {
	id, e := deviceID(r)
	if e != nil {
		bad(w, 400, "invalid id")
		return
	}
	d, e := s.scanDevice(s.DB.QueryRow(deviceSelect+" WHERE d.id=?", id))
	if e != nil {
		internalError(w, e)
		return
	}
	reply(w, 200, d)
}
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	var total, online, pending, password int
	e := s.DB.QueryRow(`SELECT count(*),COALESCE(sum(last_seen_at>=?),0),COALESCE(sum(status='pending'),0),COALESCE(sum(status='approved' AND applied_password_version<COALESCE(c.password_version,0)),0) FROM devices d LEFT JOIN device_credentials c ON c.device_id=d.id`, s.now().Unix()-120).Scan(&total, &online, &pending, &password)
	if e != nil {
		internalError(w, e)
		return
	}
	reply(w, 200, map[string]int{"total": total, "online": online, "offline": total - online, "pending": pending, "password_pending": password})
}
func (s *Server) setCredential(tx *sql.Tx, id int64) error {
	n, c, e := encrypt(s.cipher, randomToken(24), id)
	if e != nil {
		return e
	}
	now := s.now().Unix()
	_, e = tx.Exec(`INSERT INTO device_credentials VALUES(?,1,?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET password_version=password_version+1,nonce=excluded.nonce,ciphertext=excluded.ciphertext,updated_at=excluded.updated_at`, id, n, c, now, now)
	return e
}
func (s *Server) action(w http.ResponseWriter, r *http.Request) {
	id, e := deviceID(r)
	if e != nil {
		bad(w, 400, "invalid id")
		return
	}
	action := chi.URLParam(r, "action")
	tx, e := s.DB.Begin()
	if e != nil {
		internalError(w, e)
		return
	}
	defer tx.Rollback()
	var status string
	if e = tx.QueryRow("SELECT status FROM devices WHERE id=?", id).Scan(&status); e != nil {
		internalError(w, e)
		return
	}
	var audit string
	switch action {
	case "approve":
		if status == "approved" {
			bad(w, 409, "already approved")
			return
		}
		var hasKey int
		if e = tx.QueryRow("SELECT count(*) FROM device_auth WHERE device_id=?", id).Scan(&hasKey); e != nil {
			internalError(w, e)
			return
		}
		if hasKey == 0 {
			bad(w, 409, "device must enroll again after identity reset")
			return
		}
		e = s.setCredential(tx, id)
		if e == nil {
			_, e = tx.Exec("UPDATE devices SET status='approved',approved_at=?,updated_at=? WHERE id=?", s.now().Unix(), s.now().Unix(), id)
		}
		audit = "device approved"
	case "reject":
		_, e = tx.Exec("UPDATE devices SET status='rejected',updated_at=? WHERE id=?", s.now().Unix(), id)
		audit = "device rejected"
	case "rotate-password":
		if status != "approved" {
			bad(w, 409, "device is not approved")
			return
		}
		e = s.setCredential(tx, id)
		audit = "password rotated"
	case "reset-identity":
		_, e = tx.Exec("DELETE FROM device_auth WHERE device_id=?", id)
		if e == nil {
			_, e = tx.Exec("DELETE FROM agent_nonces WHERE device_id=?", id)
		}
		if e == nil {
			_, e = tx.Exec("DELETE FROM device_credentials WHERE device_id=?", id)
		}
		if e == nil {
			_, e = tx.Exec("UPDATE devices SET status='pending',approved_at=NULL,applied_policy_version=0,applied_password_version=0,updated_at=? WHERE id=?", s.now().Unix(), id)
		}
		audit = "identity reset"
	default:
		bad(w, 404, "unknown action")
		return
	}
	if e == nil {
		e = s.audit(tx, audit, id)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		internalError(w, e)
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}
func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id, e := deviceID(r)
	if e != nil {
		bad(w, 400, "invalid id")
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		internalError(w, e)
		return
	}
	defer tx.Rollback()
	res, e := tx.Exec("DELETE FROM devices WHERE id=?", id)
	if e != nil {
		internalError(w, e)
		return
	}
	n, e := res.RowsAffected()
	if e != nil {
		internalError(w, e)
		return
	}
	if n == 0 {
		bad(w, 404, "not found")
		return
	}
	if e = s.audit(tx, "device deleted", id); e == nil {
		e = tx.Commit()
	}
	if e != nil {
		internalError(w, e)
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}
func (s *Server) credential(w http.ResponseWriter, r *http.Request) {
	id, e := deviceID(r)
	if e != nil {
		bad(w, 400, "invalid id")
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		internalError(w, e)
		return
	}
	defer tx.Rollback()
	var n, c []byte
	var version int64
	e = tx.QueryRow("SELECT nonce,ciphertext,password_version FROM device_credentials c JOIN devices d ON d.id=c.device_id WHERE d.id=? AND d.status='approved'", id).Scan(&n, &c, &version)
	if e != nil {
		internalError(w, e)
		return
	}
	if len(n) != s.cipher.NonceSize() {
		bad(w, 500, "credential unavailable")
		return
	}
	p, e := s.cipher.Open(nil, n, c, []byte(strconv.FormatInt(id, 10)))
	if e != nil {
		bad(w, 500, "credential unavailable")
		return
	}
	if e = s.audit(tx, "credential revealed", id); e == nil {
		e = tx.Commit()
	}
	if e != nil {
		internalError(w, e)
		return
	}
	reply(w, 200, map[string]any{"password": string(p), "password_version": version})
}
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	p, e := s.policy(s.DB)
	if e != nil {
		internalError(w, e)
		return
	}
	reply(w, 200, p)
}
func validAddress(s string, optional bool) bool {
	if s == "" {
		return optional
	}
	if len(s) > 253 || strings.ContainsAny(s, " /\\\t\r\n@?#") {
		return false
	}
	host := s
	if strings.Contains(s, ":") {
		var port string
		var e error
		host, port, e = net.SplitHostPort(s)
		if e != nil {
			return false
		}
		p, e := strconv.Atoi(port)
		if e != nil || p < 1 || p > 65535 {
			return false
		}
	}
	if net.ParseIP(host) != nil {
		return true
	}
	for _, part := range strings.Split(host, ".") {
		if len(part) == 0 || len(part) > 63 || part[0] == '-' || part[len(part)-1] == '-' {
			return false
		}
		for _, c := range part {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var b RustDesk
	if !decode(w, r, &b) {
		return
	}
	key, e := base64.StdEncoding.DecodeString(b.Key)
	if !validAddress(b.IDServer, false) || !validAddress(b.RelayServer, true) || e != nil || len(key) != 32 || b.APIServer != "" {
		bad(w, 400, "invalid server address or public key; API server must be empty")
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		internalError(w, e)
		return
	}
	defer tx.Rollback()
	_, e = tx.Exec("UPDATE settings SET id_server=?,relay_server=?,public_key=?,policy_version=policy_version+1,updated_at=? WHERE id=1 AND (id_server<>? OR relay_server<>? OR public_key<>?)", b.IDServer, b.RelayServer, b.Key, s.now().Unix(), b.IDServer, b.RelayServer, b.Key)
	if e == nil {
		e = s.audit(tx, "RustDesk server settings changed", nil)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		internalError(w, e)
		return
	}
	s.getSettings(w, r)
}
func (s *Server) auditList(w http.ResponseWriter, r *http.Request) {
	p, n := page(r)
	var total int
	if e := s.DB.QueryRow("SELECT count(*) FROM audit_logs").Scan(&total); e != nil {
		internalError(w, e)
		return
	}
	rows, e := s.DB.Query("SELECT id,timestamp,admin,action,device,metadata FROM audit_logs ORDER BY id DESC LIMIT ? OFFSET ?", n, (p-1)*n)
	if e != nil {
		internalError(w, e)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, ts int64
		var admin, action, meta string
		var device sql.NullInt64
		if e = rows.Scan(&id, &ts, &admin, &action, &device, &meta); e != nil {
			internalError(w, e)
			return
		}
		var dev any
		if device.Valid {
			dev = device.Int64
		}
		items = append(items, map[string]any{"id": id, "timestamp": ts, "admin": admin, "action": action, "device": dev, "metadata": meta})
	}
	if e = rows.Err(); e != nil {
		internalError(w, e)
		return
	}
	reply(w, 200, map[string]any{"items": items, "total": total})
}
