// Command server is an example HTTP service that renders Pix BR Codes as
// SVG QR Codes, using only the standard library:
//
//	GET /qr.svg?payload=000201...&level=M
//
// A payload always produces the same image, so responses are cacheable
// forever: they carry a strong ETag (a hash of the payload and level) and
// Cache-Control: public, max-age=31536000, immutable. Conditional requests
// with a matching If-None-Match get 304 Not Modified without rendering.
//
//	go run ./examples/server -addr :8080
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/victorcondackdev/pix-qrcode-go/pixqr"
	"github.com/victorcondackdev/pix-qrcode-go/qr"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "listen address")
	flag.Parse()
	log.Printf("listening on http://%s/qr.svg?payload=...", *addr)
	log.Fatal(newServer(*addr).ListenAndServe())
}

func newServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           newMux(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/qr.svg", http.HandlerFunc(qrSVG))
	return mux
}

// qrSVG serves GET and HEAD /qr.svg.
func qrSVG(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	payload := q.Get("payload")
	level := qr.M
	if s := q.Get("level"); s != "" {
		l, err := qr.ParseLevel(s)
		if err != nil {
			http.Error(w, "level must be L, M, Q or H", http.StatusBadRequest)
			return
		}
		level = l
	}
	if err := pixqr.Validate(payload); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, pixqr.ErrCRC) {
			status = http.StatusUnprocessableEntity
		}
		http.Error(w, err.Error(), status)
		return
	}

	sum := sha256.Sum256([]byte(level.String() + "\x00" + payload))
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	h.Set("X-Content-Type-Options", "nosniff")
	if match(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	svg, err := pixqr.SVG(payload, pixqr.WithLevel(level))
	if err != nil { // unreachable: MaxPayload bytes always fit version 40-H
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.Set("Content-Type", "image/svg+xml")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(svg))
}

// match reports whether an If-None-Match header lists etag (or is "*").
// Weak validators match too, as RFC 9110 requires for If-None-Match.
func match(header, etag string) bool {
	for _, t := range strings.Split(header, ",") {
		t = strings.TrimPrefix(strings.TrimSpace(t), "W/")
		if t == etag || t == "*" {
			return true
		}
	}
	return false
}
