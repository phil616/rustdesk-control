// Test-only HTTPS server. Uses an ephemeral database, random encryption key and fake agent.
package main

import (
	"crypto/rand"
	"fmt"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"rustdesk-control/internal/control"
	"rustdesk-control/internal/fakeagent"
	"rustdesk-control/web"
)

func main() {
	dir, e := os.MkdirTemp("", "rdc-browser-")
	if e != nil {
		panic(e)
	}
	defer os.RemoveAll(dir)
	key := make([]byte, 32)
	rand.Read(key)
	s, e := control.Open(filepath.Join(dir, "test.db"), key, "browser-test-password")
	if e != nil {
		panic(e)
	}
	defer s.DB.Close()
	server := httptest.NewTLSServer(s.Handler(web.Handler()))
	defer server.Close()
	agent, e := fakeagent.New(server.URL, server.Client())
	if e != nil {
		panic(e)
	}
	if e = agent.Enroll(); e != nil {
		panic(e)
	}
	fmt.Println(server.URL)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}
