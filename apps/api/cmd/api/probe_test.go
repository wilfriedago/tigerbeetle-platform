package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbe(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/down" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(srv.Close)

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "healthy", url: srv.URL + "/healthz"},
		{name: "unhealthy status", url: srv.URL + "/down", wantErr: true},
		{name: "unreachable", url: "http://127.0.0.1:1/healthz", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := probe(context.Background(), tt.url, time.Second)
			if (err != nil) != tt.wantErr {
				t.Fatalf("probe(%s) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}
