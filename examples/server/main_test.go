package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const payload = "00020126330014br.gov.bcb.pix0111123456789095204000053039865406150.005802BR" +
	"5912LOJA EXEMPLO6009SAO PAULO62140510PEDIDO123463047DEC"

func get(t *testing.T, method, target string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, req)
	return rec
}

func TestQRSVG(t *testing.T) {
	target := "/qr.svg?payload=" + url.QueryEscape(payload)
	rec := get(t, http.MethodGet, target, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/svg+xml" ||
		!strings.HasPrefix(rec.Body.String(), "<svg") {
		t.Fatalf("%d %v %.60s", rec.Code, rec.Header(), rec.Body.String())
	}
	etag := rec.Header().Get("ETag")
	if len(etag) != 34 || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("caching headers %v", rec.Header())
	}

	for _, inm := range []string{etag, `"x", W/` + etag, "*"} {
		if rec := get(t, http.MethodGet, target, map[string]string{"If-None-Match": inm}); rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
			t.Fatalf("If-None-Match %s: %d", inm, rec.Code)
		}
	}
	if rec := get(t, http.MethodGet, target, map[string]string{"If-None-Match": `"other"`}); rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	h := get(t, http.MethodGet, target+"&level=H", nil)
	if h.Header().Get("ETag") == etag || h.Body.String() == rec.Body.String() {
		t.Fatal("level must change the image and the ETag")
	}
	if rec := get(t, http.MethodHead, target, nil); rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatal("HEAD")
	}
}

func TestQRSVGErrors(t *testing.T) {
	tests := []struct {
		method, target string
		code           int
	}{
		{http.MethodPost, "/qr.svg?payload=" + url.QueryEscape(payload), http.StatusMethodNotAllowed},
		{http.MethodGet, "/qr.svg", http.StatusBadRequest},
		{http.MethodGet, "/qr.svg?payload=hello", http.StatusBadRequest},
		{http.MethodGet, "/qr.svg?payload=" + url.QueryEscape(payload[:len(payload)-1]+"0"), http.StatusUnprocessableEntity},
		{http.MethodGet, "/qr.svg?level=Z&payload=" + url.QueryEscape(payload), http.StatusBadRequest},
		{http.MethodGet, "/other", http.StatusNotFound},
	}
	for _, tt := range tests {
		if rec := get(t, tt.method, tt.target, nil); rec.Code != tt.code {
			t.Errorf("%s %s: %d, want %d", tt.method, tt.target, rec.Code, tt.code)
		}
	}
}

func TestNewServer(t *testing.T) {
	srv := newServer("localhost:0")
	if srv.Addr != "localhost:0" || srv.Handler == nil || srv.ReadHeaderTimeout == 0 {
		t.Fatalf("%+v", srv)
	}
}
