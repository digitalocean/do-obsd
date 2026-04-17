// Package vpcendpoint derives the DigitalOcean VPC private service endpoint
// IP from an interface IPv4 CIDR (broadcast address minus one).
package vpcendpoint

import (
	"fmt"
	"net"
)

// FromCIDR returns the VPC endpoint IP for the IPv4 network described by cidr
// (for example "10.108.0.2/20"). The address is one less than the subnet
// broadcast address.
func FromCIDR(cidr string) (net.IP, error) {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	ip := network.IP.To4()
	if ip == nil {
		return nil, fmt.Errorf("vpcendpoint: need IPv4 CIDR, got %q", cidr)
	}
	mask := network.Mask
	if len(mask) != net.IPv4len {
		return nil, fmt.Errorf("vpcendpoint: invalid IPv4 mask in %q", cidr)
	}

	broadcast := make(net.IP, net.IPv4len)
	for i := range broadcast {
		broadcast[i] = ip[i] | ^mask[i]
	}
	broadcast[len(broadcast)-1]--
	return broadcast, nil
}
