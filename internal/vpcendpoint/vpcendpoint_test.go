package vpcendpoint

import (
	"errors"
	"net"
	"testing"
)

type fakeNetworker struct {
	ifaces []ifaceInfo
	err    error
}

func (f *fakeNetworker) interfaces() ([]ifaceInfo, error) {
	return f.ifaces, f.err
}

func TestDiscover(t *testing.T) {
	tests := []struct {
		name    string
		ifaces  []ifaceInfo
		netErr  error
		wantIP  string
		wantErr bool
	}{
		{
			name: "DO VPC example: 10.137.0.15/16 → 10.137.255.254",
			ifaces: []ifaceInfo{
				{loopback: false, addrs: []string{"10.137.0.15/16"}},
			},
			wantIP: "10.137.255.254",
		},
		{
			name: "skips loopback interface",
			ifaces: []ifaceInfo{
				{loopback: true, addrs: []string{"127.0.0.1/8"}},
				{loopback: false, addrs: []string{"10.10.0.5/24"}},
			},
			wantIP: "10.10.0.254",
		},
		{
			name: "skips interface with any public IPv4 (DO eth0 topology)",
			// eth0: public + legacy private; eth1: VPC only — must pick eth1
			ifaces: []ifaceInfo{
				{loopback: false, addrs: []string{"104.131.182.241/20", "10.17.0.5/16", "fe80::1/64"}},
				{loopback: false, addrs: []string{"10.108.0.10/20", "fe80::2/64"}},
			},
			wantIP: "10.108.15.254",
		},
		{
			name: "skips interface with only a public IPv4",
			ifaces: []ifaceInfo{
				{loopback: false, addrs: []string{"203.0.113.1/24"}},
				{loopback: false, addrs: []string{"10.0.1.1/16"}},
			},
			wantIP: "10.0.255.254",
		},
		{
			name: "skips /32 point-to-point addresses",
			ifaces: []ifaceInfo{
				{loopback: false, addrs: []string{"10.0.0.1/32"}},
				{loopback: false, addrs: []string{"192.168.1.50/24"}},
			},
			wantIP: "192.168.1.254",
		},
		{
			name: "skips IPv6 addresses",
			ifaces: []ifaceInfo{
				{loopback: false, addrs: []string{"fe80::1/64", "10.2.0.1/20"}},
			},
			wantIP: "10.2.15.254",
		},
		{
			name: "skips unparseable addresses",
			ifaces: []ifaceInfo{
				{loopback: false, addrs: []string{"not-an-ip", "172.16.0.5/12"}},
			},
			wantIP: "172.31.255.254",
		},
		{
			name: "172.16.0.0/12 range",
			ifaces: []ifaceInfo{
				{loopback: false, addrs: []string{"172.20.0.1/16"}},
			},
			wantIP: "172.20.255.254",
		},
		{
			name: "192.168 range",
			ifaces: []ifaceInfo{
				{loopback: false, addrs: []string{"192.168.0.5/24"}},
			},
			wantIP: "192.168.0.254",
		},
		{
			name:    "no suitable interface returns error",
			ifaces:  []ifaceInfo{},
			wantErr: true,
		},
		{
			name: "only loopback and public, no private",
			ifaces: []ifaceInfo{
				{loopback: true, addrs: []string{"127.0.0.1/8"}},
				{loopback: false, addrs: []string{"8.8.8.8/24"}},
			},
			wantErr: true,
		},
		{
			name:    "interfaces() returns error",
			netErr:  errors.New("netlink failure"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Discoverer{net: &fakeNetworker{ifaces: tt.ifaces, err: tt.netErr}}
			got, err := d.Discover()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got IP %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := net.ParseIP(tt.wantIP).To4()
			if !got.Equal(want) {
				t.Fatalf("got %s, want %s", got, want)
			}
		})
	}
}

func TestLastUsableHost(t *testing.T) {
	tests := []struct {
		cidr string
		want string
	}{
		{"10.137.0.0/16", "10.137.255.254"},
		{"192.168.1.0/24", "192.168.1.254"},
		{"10.0.0.0/8", "10.255.255.254"},
		{"172.16.0.0/12", "172.31.255.254"},
		{"10.10.0.0/28", "10.10.0.14"},
	}
	for _, tt := range tests {
		t.Run(tt.cidr, func(t *testing.T) {
			_, network, err := net.ParseCIDR(tt.cidr)
			if err != nil {
				t.Fatal(err)
			}
			got := lastUsableHost(network)
			want := net.ParseIP(tt.want).To4()
			if !got.Equal(want) {
				t.Fatalf("lastUsableHost(%s) = %s, want %s", tt.cidr, got, want)
			}
		})
	}
}
