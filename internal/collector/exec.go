package collector

//go:generate go tool mockgen -source=exec.go -package=collector -destination=mocks_exec_test.go

import (
	"context"
	"os/exec"
)

const (
	// sudoBin and systemctlBin must match the paths in packaging/scripts/after_install.sh sudoers rule.
	sudoBin      = "/usr/bin/sudo"
	systemctlBin = "/usr/bin/systemctl"
)

// cmdRunner abstracts exec.CommandContext for testability.
type cmdRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type realCmdRunner struct{}

func (r *realCmdRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput() //nolint:gosec
}
