package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLicenseDownloadsDoNotFallBackToSPA(t *testing.T) {
	for _, name := range []string{"source.tar.gz", "LICENSE", "THIRD-PARTY-NOTICES.txt"} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/"+name, nil))
			want, err := files.ReadFile("dist/" + name)
			if err != nil {
				if response.Code != http.StatusNotFound {
					t.Fatalf("missing legal artifact returned %d instead of 404", response.Code)
				}
				return
			}
			if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), want) {
				t.Fatalf("download is not the embedded artifact: status %d", response.Code)
			}
			if name == "source.tar.gz" && !bytes.HasPrefix(want, []byte{0x1f, 0x8b}) {
				t.Fatal("source download is not gzip")
			}
			if name == "LICENSE" && !bytes.Contains(want, []byte("GNU AFFERO GENERAL PUBLIC LICENSE")) {
				t.Fatal("missing AGPL text")
			}
		})
	}
}
