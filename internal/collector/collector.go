package collector

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	// BundlePath is where the package ships the collector binary.
	// In production this is replaced by a download from a Spaces URL
	// with sha256 verification, delivered via OpAMP.
	BundlePath = "/opt/digitalocean/bundle/do-otelcol"

	CollectorBin     = "/opt/digitalocean/bin/do-otelcol"
	CollectorService = "do-otelcol.service"

	// Start/Stop run systemctl via sudo -n so do-obsd can manage units when polkit
	// denies non-interactive sessions ("Interactive authentication required").
	sudoBin      = "/usr/bin/sudo"
	systemctlBin = "/usr/bin/systemctl"
)

//go:generate go tool mockgen -source=collector.go -package=collector -destination=mocks_test.go

// osOperator abstracts OS file operations for testability.
type osOperator interface {
	Open(name string) (io.ReadCloser, error)
	CreateTemp(dir, pattern string) (tempFile, error)
	Remove(name string) error
	Rename(oldpath, newpath string) error
}

// tempFile abstracts *os.File for testability.
type tempFile interface {
	Name() string
	Write(b []byte) (n int, err error)
	Chmod(mode os.FileMode) error
	Close() error
}

// cmdRunner abstracts exec.Command for testability.
type cmdRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

// Collector manages the do-otelcol lifecycle.
type Collector struct {
	os  osOperator
	cmd cmdRunner
}

// New returns a Collector with real OS and exec implementations.
func New() *Collector {
	return &Collector{
		os:  &realOSOperator{},
		cmd: &realCmdRunner{},
	}
}

// Install copies the bundled binary to the target path atomically.
// In production, this becomes: download from Spaces URL, verify sha256, rename.
func (c *Collector) Install() error {
	slog.Info("installing collector", "src", BundlePath, "dst", CollectorBin)

	src, err := c.os.Open(BundlePath)
	if err != nil {
		return fmt.Errorf("open bundle: %w", err)
	}
	defer func() { _ = src.Close() }()

	tmp, err := c.os.CreateTemp(filepath.Dir(CollectorBin), ".do-otelcol-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer c.os.Remove(tmpPath) //nolint:errcheck

	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := tmp.Chmod(0755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := c.os.Rename(tmpPath, CollectorBin); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	return nil
}

// Start starts do-otelcol.service via systemctl.
func (c *Collector) Start() error {
	slog.Info("starting collector", "service", CollectorService)
	out, err := c.cmd.Run(sudoBin, "-n", systemctlBin, "start", CollectorService)
	if err != nil {
		return fmt.Errorf("systemctl start %s: %w (output: %s)", CollectorService, err, out)
	}
	return nil
}

// Stop stops do-otelcol.service via systemctl.
func (c *Collector) Stop() error {
	slog.Info("stopping collector", "service", CollectorService)
	out, err := c.cmd.Run(sudoBin, "-n", systemctlBin, "stop", CollectorService)
	if err != nil {
		return fmt.Errorf("systemctl stop %s: %w (output: %s)", CollectorService, err, out)
	}
	return nil
}

// realOSOperator is the production implementation of osOperator.
type realOSOperator struct{}

func (r *realOSOperator) Open(name string) (io.ReadCloser, error) {
	return os.Open(name) //nolint:gosec
}

func (r *realOSOperator) CreateTemp(dir, pattern string) (tempFile, error) {
	return os.CreateTemp(dir, pattern)
}

func (r *realOSOperator) Remove(name string) error {
	return os.Remove(name)
}

func (r *realOSOperator) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// realCmdRunner is the production implementation of cmdRunner.
type realCmdRunner struct{}

func (r *realCmdRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput() //nolint:gosec
}
