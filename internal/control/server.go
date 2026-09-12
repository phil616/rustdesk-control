package control

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	_ "modernc.org/sqlite"
	"rustdesk-control/migrations"
)

type Server struct {
	TrustedProxies []*net.IPNet
	DB             *sql.DB
	cipher         cipher.AEAD
	now            func() time.Time
	mu             sync.Mutex
	limits         map[string]*bucket
}
type bucket struct {
	at    time.Time
	count int
}
type sessionKey struct{}
type Session struct {
	Token string
	CSRF  string
}

func Open(path string, key []byte, initialPassword string) (*Server, error) {
	a, e := newCipher(key)
	if e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	fail := func(e error) (*Server, error) { db.Close(); return nil, e }
	if _, e = db.Exec("PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;"); e != nil {
		return fail(e)
	}
	if _, e = db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY)"); e != nil {
		return fail(e)
	}
	entries, e := migrations.Files.ReadDir(".")
	if e != nil {
		return fail(e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, f := range entries {
		if !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		var exists int
		e = db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=?", f.Name()).Scan(&exists)
		if e != nil {
			return fail(e)
		}
		if exists > 0 {
			continue
		}
		b, e := migrations.Files.ReadFile(f.Name())
		if e != nil {
			return fail(e)
		}
		tx, e := db.Begin()
		if e != nil {
			return fail(e)
		}
		if _, e = tx.Exec(string(b)); e == nil {
			_, e = tx.Exec("INSERT INTO schema_migrations VALUES(?)", f.Name())
		}
		if e != nil {
			tx.Rollback()
			return fail(e)
		}
		if e = tx.Commit(); e != nil {
			return fail(e)
		}
	}
	var count int
	if e = db.QueryRow("SELECT count(*) FROM admins").Scan(&count); e != nil {
		return fail(e)
	}
	if count == 0 {
		if len(initialPassword) < 12 {
			return fail(errors.New("initial admin password must be at least 12 characters"))
		}
		if _, e = db.Exec("INSERT INTO admins VALUES(1,?)", hashPassword(initialPassword)); e != nil {
			return fail(e)
		}
	}
	return &Server{DB: db, cipher: a, now: time.Now, limits: map[string]*bucket{}}, nil
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func bad(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		bad(w, 400, "invalid JSON body")
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		bad(w, 400, "invalid JSON body")
		return false
	}
	return true
}
func (s *Server) rate(r *http.Request, scope string, max int) bool {
	host := s.clientIP(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for k, b := range s.limits {
		if now.Sub(b.at) > time.Minute {
			delete(s.limits, k)
		}
	}
	key := scope + host
	b := s.limits[key]
	if b == nil {
		if len(s.limits) >= 10000 {
			return false
		}
		b = &bucket{at: now}
		s.limits[key] = b
	}
	b.count++
	return b.count <= max
}
func (s *Server) Handler(web http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Frame-Options", "DENY")
			next.ServeHTTP(w, r)
		})
	})
	r.Get("/api/v1/agent/bootstrap", s.bootstrap)
	r.Post("/api/v1/agent/enroll", s.enroll)
	r.Post("/api/v1/agent/heartbeat", s.heartbeat)
	r.Post("/api/v1/admin/login", s.login)
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(s.authenticate)
		r.Get("/session", func(w http.ResponseWriter, r *http.Request) {
			reply(w, 200, map[string]string{"admin": "admin", "csrf_token": r.Context().Value(sessionKey{}).(Session).CSRF})
		})
		r.Post("/logout", s.logout)
		r.Put("/password", s.changePassword)
		r.Get("/dashboard", s.dashboard)
		r.Get("/devices", s.devices)
		r.Get("/devices/{id}", s.detail)
		r.Get("/devices/{id}/credential", s.credential)
		r.Post("/devices/{id}/{action}", s.action)
		r.Delete("/devices/{id}", s.deleteDevice)
		r.Get("/settings/rustdesk", s.getSettings)
		r.Put("/settings/rustdesk", s.putSettings)
		r.Get("/audit", s.auditList)
	})
	r.HandleFunc("/api/*", func(w http.ResponseWriter, r *http.Request) { bad(w, 404, "not found") })
	if web != nil {
		r.Handle("/*", web)
	}
	return r
}
func sameOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	if o := r.Header.Get("Origin"); o != "" {
		u, e := url.Parse(o)
		return e == nil && u.Scheme == "https" && u.Host == r.Host
	}
	return true
}
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie("rdc_session")
		if e != nil {
			bad(w, 401, "authentication required")
			return
		}
		sess := Session{Token: tokenHash(c.Value)}
		if e = s.DB.QueryRow("SELECT csrf FROM admin_sessions WHERE token_hash=? AND expires_at>?", sess.Token, s.now().Unix()).Scan(&sess.CSRF); e != nil {
			bad(w, 401, "session expired")
			return
		}
		if r.Method != "GET" && (!sameOrigin(r) || r.Header.Get("X-CSRF-Token") != sess.CSRF) {
			bad(w, 403, "CSRF validation failed")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, sess)))
	})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		bad(w, 403, "origin rejected")
		return
	}
	if !s.rate(r, "login", 5) {
		bad(w, 429, "try again later")
		return
	}
	var b struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &b) {
		return
	}
	var h string
	if s.DB.QueryRow("SELECT password_hash FROM admins WHERE id=1").Scan(&h) != nil || !checkPassword(b.Password, h) {
		bad(w, 401, "invalid credentials")
		return
	}
	token := randomToken(32)
	csrf := randomToken(32)
	tx, e := s.DB.Begin()
	if e != nil {
		bad(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	_, e = tx.Exec("DELETE FROM admin_sessions WHERE expires_at<=?", s.now().Unix())
	if e == nil {
		_, e = tx.Exec("INSERT INTO admin_sessions VALUES(?,?,?)", tokenHash(token), csrf, s.now().Add(24*time.Hour).Unix())
	}
	if e == nil {
		e = s.audit(tx, "admin login", nil)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		bad(w, 500, "database error")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "rdc_session", Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: 86400})
	reply(w, 200, map[string]string{"csrf_token": csrf})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	_, e := s.DB.Exec("DELETE FROM admin_sessions WHERE token_hash=?", r.Context().Value(sessionKey{}).(Session).Token)
	if e != nil {
		bad(w, 500, "database error")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "rdc_session", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	reply(w, 200, map[string]bool{"ok": true})
}
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !s.rate(r, "password", 5) {
		bad(w, 429, "try again later")
		return
	}
	var h string
	if s.DB.QueryRow("SELECT password_hash FROM admins WHERE id=1").Scan(&h) != nil || !checkPassword(b.Current, h) {
		bad(w, 403, "invalid current password")
		return
	}
	if len(b.New) < 12 {
		bad(w, 400, "password must be at least 12 characters")
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		bad(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	_, e = tx.Exec("UPDATE admins SET password_hash=? WHERE id=1", hashPassword(b.New))
	if e == nil {
		_, e = tx.Exec("DELETE FROM admin_sessions")
	}
	if e == nil {
		e = s.audit(tx, "admin password changed", nil)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		bad(w, 500, "database error")
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}
func (s *Server) audit(tx *sql.Tx, action string, id any) error {
	_, e := tx.Exec("INSERT INTO audit_logs(timestamp,admin,action,device) VALUES(?,'admin',?,?)", s.now().Unix(), action, id)
	return e
}
func deviceID(r *http.Request) (int64, error) { return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64) }
func internalError(w http.ResponseWriter, e error) {
	if errors.Is(e, sql.ErrNoRows) {
		bad(w, 404, "not found")
	} else {
		bad(w, 500, "database error")
	}
}

// Only explicitly configured proxy peers may supply forwarding information.
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	trusted := func(host string) bool {
		ip := net.ParseIP(host)
		for _, network := range s.TrustedProxies {
			if network.Contains(ip) {
				return true
			}
		}
		return false
	}
	if !trusted(host) {
		return host
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(chain[i])
		if net.ParseIP(candidate) == nil {
			return host
		}
		host = candidate
		if !trusted(host) {
			return host
		}
	}
	return host
}
