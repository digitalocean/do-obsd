package collector

import _ "embed"

// GPUConfig is the bootstrap otelcol configuration for GPU droplet instances.
// In the future, instance-type-specific configurations will be delivered at
// runtime via OpAMP, making this the fallback for first boot only.
//
//go:embed gpu_config.yaml
var GPUConfig []byte
