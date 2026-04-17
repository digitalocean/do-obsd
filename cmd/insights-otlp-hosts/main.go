package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/digitalocean/do-obsd/internal/vpcendpoint"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ifaceName := flag.String("iface", "eth1", "network interface to read IPv4 address from")
	hostname := flag.String("hostname", "insights-otlp.digitalocean.com", "hostname for /etc/hosts line")
	hostsPath := flag.String("hosts", "/etc/hosts", "hosts file path (with -apply)")
	apply := flag.Bool("apply", false, "append mapping to hosts file if missing (requires root)")
	quiet := flag.Bool("q", false, "print only the VPC endpoint IP (for scripts)")
	flag.Parse()

	cidr, err := primaryIPv4CIDR(*ifaceName)
	if err != nil {
		return err
	}

	endpoint, err := vpcendpoint.FromCIDR(cidr)
	if err != nil {
		return fmt.Errorf("vpc endpoint: %w", err)
	}
	ip := endpoint.String()

	if *quiet {
		fmt.Println(ip)
	} else {
		fmt.Printf("cidr (from %s): %s\nvpc-endpoint: %s\n\n", *ifaceName, cidr, ip)
		fmt.Printf("hosts line:\n%s\t%s\n", ip, *hostname)
	}

	if !*apply {
		return nil
	}

	line := fmt.Sprintf("%s\t%s\n", ip, *hostname)
	return appendHostsEntry(*hostsPath, *hostname, ip, line)
}

func primaryIPv4CIDR(ifaceName string) (string, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("interface %q: %w", ifaceName, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("addrs %q: %w", ifaceName, err)
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.To4() == nil {
			continue
		}
		return ipnet.String(), nil
	}
	return "", fmt.Errorf("no IPv4 address on %q", ifaceName)
}

func appendHostsEntry(path, hostname, wantIP, line string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	scanErr := scanHostsForConflict(f, hostname, wantIP)
	if closeErr := f.Close(); closeErr != nil && scanErr == nil {
		return closeErr
	}
	if scanErr != nil {
		if errors.Is(scanErr, errAlreadyPresent) {
			return nil
		}
		return scanErr
	}

	out, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := out.WriteString(line); err != nil {
		return err
	}
	slog.Info("appended hosts entry", "path", path, "ip", wantIP, "hostname", hostname)
	return nil
}

func scanHostsForConflict(r io.Reader, hostname, wantIP string) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := fields[0]
		if strings.Contains(ip, ":") {
			continue
		}
		for _, name := range fields[1:] {
			if name != hostname {
				continue
			}
			if ip == wantIP {
				slog.Info("hosts entry already present", "hostname", hostname, "ip", wantIP)
				return errAlreadyPresent
			}
			return fmt.Errorf("hosts already maps %q to %s (refusing to add %s)", hostname, ip, wantIP)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}

var errAlreadyPresent = errors.New("hosts entry already present")
