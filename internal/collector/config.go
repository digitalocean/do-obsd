package collector

import (
	_ "embed"
	"bytes"
	"fmt"
	"net"
	"text/template"
)

//go:embed config.yaml.tmpl
var configTmpl string

var parsedConfigTmpl = template.Must(template.New("config").Parse(configTmpl))

// BuildConfig renders the collector config with the given VPC endpoint IP.
// The exporter endpoint is set to <ip>:443.
func BuildConfig(ip net.IP) ([]byte, error) {
	data := struct {
		ExporterEndpoint string
	}{
		ExporterEndpoint: fmt.Sprintf("%s:443", ip),
	}
	var buf bytes.Buffer
	if err := parsedConfigTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render config: %w", err)
	}
	return buf.Bytes(), nil
}
