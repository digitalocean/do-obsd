package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckHealth(t *testing.T) {
	tests := []struct {
		name      string
		handler   http.HandlerFunc
		wantErr   bool
		errSubstr string
	}{
		{
			name: "healthy - 200 OK",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name: "unhealthy - 503",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			},
			wantErr:   true,
			errSubstr: "status 503",
		},
		{
			name: "unhealthy - 500",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr:   true,
			errSubstr: "status 500",
		},
		{
			name: "timeout",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(5 * time.Second)
				w.WriteHeader(http.StatusOK)
			},
			wantErr:   true,
			errSubstr: "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			c := &Collector{
				http:      srv.Client(),
				healthURL: srv.URL + "/",
			}

			err := c.CheckHealth(context.Background())

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if tt.errSubstr != "" && err != nil && !strings.Contains(err.Error(), tt.errSubstr) {
				t.Fatalf("expected error containing %q, got: %v", tt.errSubstr, err)
			}
		})
	}
}

func TestCheckHealth_ConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srvURL := srv.URL
	srv.Close()

	c := &Collector{
		http:      &http.Client{},
		healthURL: srvURL + "/",
	}

	err := c.CheckHealth(context.Background())
	if err == nil {
		t.Fatal("expected connection refused error, got nil")
	}
	if !strings.Contains(err.Error(), "health check request") {
		t.Fatalf("expected health check request error, got: %v", err)
	}
}

func TestCheckHealth_RedirectRejected(t *testing.T) {
	redirectSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://evil.example.com/", http.StatusFound)
	}))
	defer redirectSrv.Close()

	c := &Collector{
		http: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		healthURL: redirectSrv.URL + "/",
	}

	err := c.CheckHealth(context.Background())
	if err == nil {
		t.Fatal("expected error for redirect, got nil")
	}
	if !strings.Contains(err.Error(), "status 302") {
		t.Fatalf("expected status 302 error, got: %v", err)
	}
}
