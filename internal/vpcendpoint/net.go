package vpcendpoint

import "net"

type ifaceInfo struct {
	loopback bool
	addrs    []string // CIDR strings, e.g. "10.137.0.15/16"
}

type networker interface {
	interfaces() ([]ifaceInfo, error)
}

type realNetworker struct{}

func (r *realNetworker) interfaces() ([]ifaceInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := make([]ifaceInfo, 0, len(ifaces))
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		strs := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			strs = append(strs, addr.String())
		}
		result = append(result, ifaceInfo{
			loopback: iface.Flags&net.FlagLoopback != 0,
			addrs:    strs,
		})
	}
	return result, nil
}
