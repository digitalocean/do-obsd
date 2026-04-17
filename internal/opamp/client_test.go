package opamp

import (
	"reflect"
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
)

func TestExtractRemoteConfig(t *testing.T) {
	tests := []struct {
		name    string
		remote  *protobufs.AgentRemoteConfig
		want    []byte
		wantErr bool
	}{
		{
			name: "single empty-key config",
			remote: &protobufs.AgentRemoteConfig{
				Config: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{
						"": {Body: []byte("service:\n  pipelines: {}\n")},
					},
				},
			},
			want: []byte("service:\n  pipelines: {}\n"),
		},
		{
			name: "single named section",
			remote: &protobufs.AgentRemoteConfig{
				Config: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{
						"default": {Body: []byte("receivers: {}\n")},
					},
				},
			},
			want: []byte("receivers: {}\n"),
		},
		{
			name: "multiple sections unsupported",
			remote: &protobufs.AgentRemoteConfig{
				Config: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{
						"a": {Body: []byte("a: 1\n")},
						"b": {Body: []byte("b: 2\n")},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing payload",
			remote: &protobufs.AgentRemoteConfig{
				Config: &protobufs.AgentConfigMap{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractRemoteConfig(tt.remote)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %q, want %q", string(got), string(tt.want))
			}
		})
	}
}

func TestInstanceUID(t *testing.T) {
	id1, err := instanceUID()
	if err != nil {
		t.Fatalf("instanceUID() error: %v", err)
	}
	id2, err := instanceUID()
	if err != nil {
		t.Fatalf("instanceUID() second call error: %v", err)
	}
	if id1 != id2 {
		t.Fatal("instanceUID should be stable across calls")
	}
}
