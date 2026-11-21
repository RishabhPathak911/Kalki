package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRenderTransportBoundary(t *testing.T) {
	h := renderHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			t.Error("downstream transport missing")
		}
		w.WriteHeader(204)
	}), nil)
	for _, tc := range []struct {
		path, proto string
		code        int
	}{{"/healthz", "", 200}, {"/api/ingest", "", 426}, {"/api/ingest", "http", 426}, {"/api/ingest", "https", 204}} {
		r := httptest.NewRequest("GET", "http://internal"+tc.path, nil)
		r.Header.Set("X-Forwarded-Proto", tc.proto)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s: %d", tc.path, w.Code)
		}
		if r.TLS != nil {
			t.Fatal("original request mutated")
		}
	}
}
