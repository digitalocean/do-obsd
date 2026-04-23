package supervisor

import (
	"net"
	"strings"
	"testing"
)

func TestBuildConfig(t *testing.T) {
	tests := []struct {
		name         string
		ip           net.IP
		wantEndpoint string
	}{
		{
			name:         "DO VPC example",
			ip:           net.ParseIP("10.137.255.254"),
			wantEndpoint: "10.137.255.254:443",
		},
		{
			name:         "192.168 range",
			ip:           net.ParseIP("192.168.1.254"),
			wantEndpoint: "192.168.1.254:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildConfig(tt.ip)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s := string(got)
			if !strings.Contains(s, "endpoint: "+tt.wantEndpoint) {
				t.Fatalf("endpoint %q not found in config:\n%s", tt.wantEndpoint, s)
			}
			if !strings.Contains(s, "Host: insights-otlp.digitalocean.com") {
				t.Fatalf("Host header not found in config:\n%s", s)
			}
			if !strings.Contains(s, "server_name_override: insights-otlp.digitalocean.com") {
				t.Fatalf("server_name_override not found in config:\n%s", s)
			}
		})
	}
}
