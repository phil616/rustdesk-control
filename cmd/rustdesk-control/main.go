package main

import (
	"context"
	"encoding/base64"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"rustdesk-control/internal/control"
	"rustdesk-control/web"
)

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address; terminate HTTPS at trusted reverse proxy")
	db := flag.String("database", "./data/rustdesk-control.db", "SQLite database path")
	trusted := flag.String("trusted-proxies", "", "comma-separated trusted proxy CIDRs; forwarding headers are otherwise ignored")
	flag.Parse()
	key, e := base64.StdEncoding.DecodeString(os.Getenv("RUSTDESK_CONTROL_MASTER_KEY"))
	if e != nil || len(key) != 32 {
		log.Fatal("RUSTDESK_CONTROL_MASTER_KEY must be a base64-encoded 32-byte key")
	}
	if e = os.MkdirAll(filepath.Dir(*db), 0700); e != nil {
		log.Fatal(e)
	}
	f, e := os.OpenFile(*db, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		log.Fatal(e)
	}
	f.Close()
	if e = os.Chmod(*db, 0600); e != nil {
		log.Fatal(e)
	}
	s, e := control.Open(*db, key, os.Getenv("RUSTDESK_CONTROL_ADMIN_PASSWORD"))
	if e != nil {
		log.Fatal(e)
	}
	defer s.DB.Close()
	for _, cidr := range strings.Split(*trusted, ",") {
		if cidr == "" {
			continue
		}
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			log.Fatal("invalid trusted proxy CIDR")
		}
		s.TrustedProxies = append(s.TrustedProxies, network)
	}
	srv := &http.Server{Addr: *listen, Handler: s.Handler(web.Handler()), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()
	log.Printf("rustdesk-control listening on %s", *listen)
	if e = srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
