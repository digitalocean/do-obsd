package vpcendpoint

import (
	"net"
	"testing"
)

func TestFromCIDR(t *testing.T) {
	tests := []struct {
		cidr string
		want string
	}{
		{"10.108.0.2/20", "10.108.15.254"},
		{"10.116.0.2/20", "10.116.15.254"},
		{"192.0.2.10/24", "192.0.2.254"},
	}
	for _, tt := range tests {
		t.Run(tt.cidr, func(t *testing.T) {
			got, err := FromCIDR(tt.cidr)
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != tt.want {
				t.Fatalf("FromCIDR(%q) = %s; want %s", tt.cidr, got, tt.want)
			}
		})
	}
}

func TestFromCIDR_errors(t *testing.T) {
	_, err := FromCIDR("not-a-cidr")
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = FromCIDR("2001:db8::1/64")
	if err == nil {
		t.Fatal("expected error for IPv6")
	}
}

func TestFromCIDR_usesNetworkBits(t *testing.T) {
	// ParseCIDR normalizes to network address; result must still match any host in subnet.
	got1, err := FromCIDR("10.108.0.0/20")
	if err != nil {
		t.Fatal(err)
	}
	got2, err := FromCIDR("10.108.15.255/20")
	if err != nil {
		t.Fatal(err)
	}
	want := net.ParseIP("10.108.15.254").To4()
	if !got1.Equal(want) || !got2.Equal(want) {
		t.Fatalf("got %v and %v; want %v", got1, got2, want)
	}
}
