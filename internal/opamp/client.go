package opamp

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/open-telemetry/opamp-go/client"
	opamptypes "github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
)

const (
	agentType       = "com.digitalocean.do-obsd"
	machineIDPath   = "/etc/machine-id"
	hostnameKey     = "host.name"
	serviceNameKey  = "service.name"
	serviceVerKey   = "service.version"
	emptyConfigName = ""
)

// ConfigWriter abstracts applying collector configuration.
type ConfigWriter interface {
	WriteConfig(data []byte) error
}

// Client manages OpAMP connectivity and remote config application.
type Client struct {
	version   string
	serverURL string
	writer    ConfigWriter
	client    client.OpAMPClient

	mu              sync.RWMutex
	effectiveConfig []byte
}

// NewFromEnv creates an OpAMP client when DO_OBSD_OPAMP_SERVER_URL is set.
// If the variable is empty, it returns nil, nil.
func NewFromEnv(version string, writer ConfigWriter, bootstrapConfig []byte) (*Client, error) {
	serverURL := strings.TrimSpace(os.Getenv("DO_OBSD_OPAMP_SERVER_URL"))
	if serverURL == "" {
		return nil, nil
	}
	if writer == nil {
		return nil, errors.New("opamp: nil config writer")
	}

	initial := make([]byte, len(bootstrapConfig))
	copy(initial, bootstrapConfig)

	return &Client{
		version:         version,
		serverURL:       serverURL,
		writer:          writer,
		client:          client.NewWebSocket(&logger{}),
		effectiveConfig: initial,
	}, nil
}

func (c *Client) Start(ctx context.Context) error {
	instanceID, err := instanceUID()
	if err != nil {
		return fmt.Errorf("instance uid: %w", err)
	}

	if err := c.client.SetAgentDescription(c.agentDescription()); err != nil {
		return fmt.Errorf("set agent description: %w", err)
	}
	if err := c.client.SetHealth(&protobufs.ComponentHealth{Healthy: true}); err != nil {
		return fmt.Errorf("set health: %w", err)
	}

	settings := opamptypes.StartSettings{
		OpAMPServerURL: c.serverURL,
		InstanceUid:    instanceID,
		Callbacks: opamptypes.Callbacks{
			OnConnect: func(ctx context.Context) {
				slog.InfoContext(ctx, "opamp connected", "server", c.serverURL)
			},
			OnConnectFailed: func(ctx context.Context, err error) {
				slog.ErrorContext(ctx, "opamp connect failed", "err", err)
			},
			OnError: func(ctx context.Context, err *protobufs.ServerErrorResponse) {
				if err != nil {
					slog.ErrorContext(ctx, "opamp server error", "message", err.ErrorMessage)
				}
			},
			GetEffectiveConfig: func(context.Context) (*protobufs.EffectiveConfig, error) {
				c.mu.RLock()
				body := make([]byte, len(c.effectiveConfig))
				copy(body, c.effectiveConfig)
				c.mu.RUnlock()

				return &protobufs.EffectiveConfig{
					ConfigMap: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							emptyConfigName: {Body: body},
						},
					},
				}, nil
			},
			OnMessage: func(ctx context.Context, msg *opamptypes.MessageData) {
				c.onMessage(ctx, msg)
			},
		},
		Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth,
	}

	if err := c.client.Start(ctx, settings); err != nil {
		return fmt.Errorf("start opamp client: %w", err)
	}
	slog.Info("opamp started", "server", c.serverURL)
	return nil
}

func (c *Client) Stop(ctx context.Context) error {
	if err := c.client.SetHealth(&protobufs.ComponentHealth{
		Healthy:   false,
		LastError: "do-obsd shutdown",
	}); err != nil {
		slog.Debug("opamp set shutdown health failed", "err", err)
	}
	return c.client.Stop(ctx)
}

func (c *Client) onMessage(ctx context.Context, msg *opamptypes.MessageData) {
	if msg == nil || msg.RemoteConfig == nil {
		return
	}

	configBody, err := extractRemoteConfig(msg.RemoteConfig)
	status := &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: msg.RemoteConfig.ConfigHash,
	}
	if err != nil {
		status.Status = protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED
		status.ErrorMessage = err.Error()
		_ = c.client.SetRemoteConfigStatus(status)
		slog.ErrorContext(ctx, "opamp remote config rejected", "err", err)
		return
	}

	if err := c.writer.WriteConfig(configBody); err != nil {
		status.Status = protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED
		status.ErrorMessage = err.Error()
		_ = c.client.SetRemoteConfigStatus(status)
		slog.ErrorContext(ctx, "opamp remote config apply failed", "err", err)
		return
	}

	c.mu.Lock()
	c.effectiveConfig = make([]byte, len(configBody))
	copy(c.effectiveConfig, configBody)
	c.mu.Unlock()

	status.Status = protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED
	if err := c.client.SetRemoteConfigStatus(status); err != nil {
		slog.ErrorContext(ctx, "opamp set remote config status failed", "err", err)
	}
	if err := c.client.UpdateEffectiveConfig(ctx); err != nil {
		slog.ErrorContext(ctx, "opamp update effective config failed", "err", err)
	}
	slog.InfoContext(ctx, "opamp remote config applied")
}

func (c *Client) agentDescription() *protobufs.AgentDescription {
	hostname, _ := os.Hostname()
	return &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			keyVal(serviceNameKey, agentType),
			keyVal(serviceVerKey, c.version),
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			keyVal("os.type", runtime.GOOS),
			keyVal(hostnameKey, hostname),
		},
	}
}

func keyVal(key, val string) *protobufs.KeyValue {
	return &protobufs.KeyValue{
		Key: key,
		Value: &protobufs.AnyValue{
			Value: &protobufs.AnyValue_StringValue{StringValue: val},
		},
	}
}

func extractRemoteConfig(remote *protobufs.AgentRemoteConfig) ([]byte, error) {
	if remote == nil || remote.Config == nil || len(remote.Config.ConfigMap) == 0 {
		return nil, errors.New("missing remote config payload")
	}

	if cfg, ok := remote.Config.ConfigMap[emptyConfigName]; ok && cfg != nil && len(cfg.Body) > 0 {
		out := make([]byte, len(cfg.Body))
		copy(out, cfg.Body)
		return out, nil
	}

	keys := make([]string, 0, len(remote.Config.ConfigMap))
	for k := range remote.Config.ConfigMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) != 1 {
		return nil, fmt.Errorf("unsupported multi-part remote config sections: %v", keys)
	}

	cfg := remote.Config.ConfigMap[keys[0]]
	if cfg == nil || len(cfg.Body) == 0 {
		return nil, fmt.Errorf("remote config section %q is empty", keys[0])
	}
	out := make([]byte, len(cfg.Body))
	copy(out, cfg.Body)
	return out, nil
}

func instanceUID() (opamptypes.InstanceUid, error) {
	source, err := os.ReadFile(machineIDPath)
	if err != nil {
		host, hostErr := os.Hostname()
		if hostErr != nil {
			return opamptypes.InstanceUid{}, fmt.Errorf("read machine-id: %w (hostname fallback failed: %v)", err, hostErr)
		}
		source = []byte(host)
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(source))))

	var id opamptypes.InstanceUid
	copy(id[:], sum[:16])
	return id, nil
}

type logger struct{}

func (l *logger) Debugf(ctx context.Context, format string, v ...interface{}) {
	slog.DebugContext(ctx, fmt.Sprintf(format, v...))
}

func (l *logger) Errorf(ctx context.Context, format string, v ...interface{}) {
	slog.ErrorContext(ctx, fmt.Sprintf(format, v...))
}
