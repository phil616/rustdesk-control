package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"rustdesk-control/internal/fakeagent"
)

func main() {
	base := flag.String("url", "", "HTTPS control origin")
	once := flag.Bool("once", false, "enroll and heartbeat once")
	flag.Parse()
	u, e := url.Parse(*base)
	if e != nil || u.Scheme != "https" || u.Host == "" {
		log.Fatal("--url must be HTTPS")
	}
	a, e := fakeagent.New(*base, &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }})
	if e != nil {
		log.Fatal(e)
	}
	if e = a.Enroll(); e != nil {
		log.Fatal(e)
	}
	log.Printf("Fake agent enrolled: %s (no real remote access)", a.UUID)
	for {
		status, e := a.Heartbeat()
		if e != nil {
			log.Print(e)
			os.Exit(1)
		}
		log.Printf("status=%s policy_version=%d password_version=%d", status, a.PolicyVersion, a.PasswordVersion)
		if *once {
			return
		}
		time.Sleep(time.Minute)
	}
}
