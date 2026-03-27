package collector

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

func newBinaryDownloadHTTPClient() *http.Client {
	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 2 * time.Minute,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	return &http.Client{Transport: t}
}
