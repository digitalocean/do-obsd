package supervisor

import (
	"bytes"
	_ "embed"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"text/template"
)

const collectorConfigPath = "/etc/do-otelcol/config.yaml"

//go:embed collector_config.yaml.tmpl
var collectorConfigTmpl string

var parsedCollectorConfigTmpl = template.Must(template.New("collector_config").Parse(collectorConfigTmpl))

func buildCollectorConfig(ip net.IP) ([]byte, error) {
	data := struct {
		ExporterEndpoint string
	}{
		ExporterEndpoint: net.JoinHostPort(ip.String(), "443"),
	}
	var buf bytes.Buffer
	if err := parsedCollectorConfigTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render config: %w", err)
	}
	return buf.Bytes(), nil
}

// writeCollectorConfig atomically writes data to collectorConfigPath.
// The temp-file-then-rename sequence ensures do-otelcol never reads a partial write.
func (s *Supervisor) writeCollectorConfig(data []byte) error {
	slog.Info("writing collector config", "path", collectorConfigPath)

	tmp, err := s.os.CreateTemp(filepath.Dir(collectorConfigPath), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer s.os.Remove(tmpPath) //nolint:errcheck

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Chmod(0640); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := s.os.Rename(tmpPath, collectorConfigPath); err != nil {
		return fmt.Errorf("install config: %w", err)
	}
	return nil
}
