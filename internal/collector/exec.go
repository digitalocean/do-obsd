package collector

//go:generate go tool mockgen -source=exec.go -package=collector -destination=mocks_exec_test.go

import "os/exec"

const (
	// sudoBin and systemctlBin must match the paths in packaging/scripts/after_install.sh sudoers rule.
	sudoBin      = "/usr/bin/sudo"
	systemctlBin = "/usr/bin/systemctl"
)

// cmdRunner abstracts exec.Command for testability.
type cmdRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

type realCmdRunner struct{}

func (r *realCmdRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput() //nolint:gosec
}
