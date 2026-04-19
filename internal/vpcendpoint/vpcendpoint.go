package vpcendpoint

import (
	"fmt"
	"net"
)

var privateRanges = []*net.IPNet{
	mustParseCIDR("10.0.0.0/8"),
	mustParseCIDR("172.16.0.0/12"),
	mustParseCIDR("192.168.0.0/16"),
}

// Discoverer finds the VPC metadata endpoint IP from network interfaces.
type Discoverer struct {
	net networker
}

// New returns a Discoverer using real network interfaces.
func New() *Discoverer {
	return &Discoverer{net: &realNetworker{}}
}

// Discover returns the VPC endpoint IP: the last usable host address in the
// subnet of the first interface that carries only private IPv4 addresses.
//
// Interfaces with any public IPv4 address are skipped — on DigitalOcean,
// eth0 carries both the public IP and a legacy private address, while the
// VPC interface (eth1, ens4, etc.) carries only private addresses.
// Interface name is not assumed.
func (d *Discoverer) Discover() (net.IP, error) {
	ifaces, err := d.net.interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if iface.loopback {
			continue
		}
		if hasPublicIPv4(iface.addrs) {
			continue
		}
		for _, cidr := range iface.addrs {
			ip, network, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			if ip.To4() == nil {
				continue // skip IPv6
			}
			if !isPrivate(ip) {
				continue
			}
			ones, bits := network.Mask.Size()
			if ones == bits {
				continue // skip /32 point-to-point addresses
			}
			return lastUsableHost(network), nil
		}
	}
	return nil, fmt.Errorf("no non-loopback private-only IPv4 interface with subnet found")
}

// hasPublicIPv4 reports whether any address in addrs is a routable public IPv4.
func hasPublicIPv4(addrs []string) bool {
	for _, cidr := range addrs {
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ip.To4() == nil {
			continue
		}
		if !isPrivate(ip) && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

// lastUsableHost returns broadcast-1 for the given network.
// For 10.137.0.0/16: broadcast=10.137.255.255, last usable=10.137.255.254.
func lastUsableHost(network *net.IPNet) net.IP {
	ip := network.IP.To4()
	mask := []byte(network.Mask)
	broadcast := make(net.IP, 4)
	for i := range 4 {
		broadcast[i] = ip[i] | ^mask[i]
	}
	result := make(net.IP, 4)
	copy(result, broadcast)
	for i := 3; i >= 0; i-- {
		result[i]--
		if result[i] != 0xFF {
			break
		}
	}
	return result
}

func isPrivate(ip net.IP) bool {
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}
