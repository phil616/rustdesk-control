package control

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func tokenHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func hashPassword(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	h := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return base64.StdEncoding.EncodeToString(salt) + ":" + base64.StdEncoding.EncodeToString(h)
}
func checkPassword(password, stored string) bool {
	p := strings.Split(stored, ":")
	if len(p) != 2 {
		return false
	}
	s, e := base64.StdEncoding.DecodeString(p[0])
	if e != nil {
		return false
	}
	want, e := base64.StdEncoding.DecodeString(p[1])
	if e != nil || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), s, 3, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}
func newCipher(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must contain 32 bytes")
	}
	b, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(b)
}
func encrypt(a cipher.AEAD, password string, id int64) ([]byte, []byte, error) {
	n := make([]byte, a.NonceSize())
	if _, e := rand.Read(n); e != nil {
		return nil, nil, e
	}
	return n, a.Seal(nil, n, []byte(password), []byte(strconv.FormatInt(id, 10))), nil
}
func Canonical(method, path, timestamp, nonce string, body []byte) []byte {
	h := sha256.Sum256(body)
	return []byte(strings.Join([]string{method, path, timestamp, nonce, hex.EncodeToString(h[:])}, "\n"))
}
func validSignature(key []byte, method, path, ts, nonce, sig string, body []byte, now time.Time) bool {
	t, e := strconv.ParseInt(ts, 10, 64)
	if e != nil || t < now.Unix()-300 || t > now.Unix()+300 || len(nonce) < 16 || len(nonce) > 128 || strings.ContainsAny(nonce, "\r\n") {
		return false
	}
	b, e := base64.StdEncoding.DecodeString(sig)
	return e == nil && len(key) == ed25519.PublicKeySize && ed25519.Verify(key, Canonical(method, path, ts, nonce, body), b)
}
