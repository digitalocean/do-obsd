package collector

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
)

// TestCollectorServiceUnitName guards the systemd unit managed via sudo (packaging sudoers allows only this unit).
func TestCollectorServiceUnitName(t *testing.T) {
	if CollectorService != "do-otelcol.service" {
		t.Fatalf("CollectorService = %q, want do-otelcol.service", CollectorService)
	}
}

// TestSudoBinPath and TestSystemctlBinPath guard the absolute paths used in the sudo invocation.
// The sudoers drop-in installed by after_install.sh grants access by exact path — a relative or
// wrong path silently breaks privilege escalation at runtime without a compile-time signal.
func TestSudoBinPath(t *testing.T) {
	if sudoBin != "/usr/bin/sudo" {
		t.Fatalf("sudoBin = %q, want /usr/bin/sudo — must match path in packaging/scripts/after_install.sh sudoers rule", sudoBin)
	}
}

func TestSystemctlBinPath(t *testing.T) {
	if systemctlBin != "/usr/bin/systemctl" {
		t.Fatalf("systemctlBin = %q, want /usr/bin/systemctl — must match path in packaging/scripts/after_install.sh sudoers rule", systemctlBin)
	}
}

func TestInstall(t *testing.T) {
	type args struct {
		os  *MockosOperator
		tmp *MocktempFile
	}

	tests := []struct {
		name    string
		expects func(*args) error
	}{
		{
			name: "happy path",
			expects: func(a *args) error {
				a.os.EXPECT().Open(BundlePath).Return(io.NopCloser(strings.NewReader("binary")), nil)
				a.os.EXPECT().CreateTemp(filepath.Dir(CollectorBin), gomock.Any()).Return(a.tmp, nil)
				a.tmp.EXPECT().Name().Return("/tmp/.do-otelcol-test").AnyTimes()
				a.tmp.EXPECT().Write(gomock.Any()).Return(6, nil)
				a.tmp.EXPECT().Chmod(os.FileMode(0755)).Return(nil)
				a.tmp.EXPECT().Close().Return(nil)
				a.os.EXPECT().Rename("/tmp/.do-otelcol-test", CollectorBin).Return(nil)
				a.os.EXPECT().Remove("/tmp/.do-otelcol-test").Return(nil)
				return nil
			},
		},
		{
			name: "bundle not found",
			expects: func(a *args) error {
				a.os.EXPECT().Open(BundlePath).Return(nil, os.ErrNotExist)
				return os.ErrNotExist
			},
		},
		{
			name: "rename fails",
			expects: func(a *args) error {
				renameErr := errors.New("cross-device link")
				a.os.EXPECT().Open(BundlePath).Return(io.NopCloser(strings.NewReader("binary")), nil)
				a.os.EXPECT().CreateTemp(filepath.Dir(CollectorBin), gomock.Any()).Return(a.tmp, nil)
				a.tmp.EXPECT().Name().Return("/tmp/.do-otelcol-test").AnyTimes()
				a.tmp.EXPECT().Write(gomock.Any()).Return(6, nil)
				a.tmp.EXPECT().Chmod(os.FileMode(0755)).Return(nil)
				a.tmp.EXPECT().Close().Return(nil)
				a.os.EXPECT().Rename("/tmp/.do-otelcol-test", CollectorBin).Return(renameErr)
				a.os.EXPECT().Remove("/tmp/.do-otelcol-test").Return(nil)
				return renameErr
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockOS := NewMockosOperator(ctrl)
			mockTmp := NewMocktempFile(ctrl)

			c := &Collector{os: mockOS, cmd: nil}
			expectedErr := tt.expects(&args{mockOS, mockTmp})

			err := c.Install()

			if !errors.Is(err, expectedErr) {
				t.Fatalf("expected error %v, got %v", expectedErr, err)
			}
		})
	}
}

func TestStart(t *testing.T) {
	tests := []struct {
		name    string
		expects func(*MockcmdRunner) error
	}{
		{
			name: "happy path",
			expects: func(cmd *MockcmdRunner) error {
				cmd.EXPECT().Run(sudoBin, "-n", systemctlBin, "start", CollectorService).Return(nil, nil)
				return nil
			},
		},
		{
			name: "systemctl fails",
			expects: func(cmd *MockcmdRunner) error {
				cmdErr := errors.New("exit status 1")
				cmd.EXPECT().Run(sudoBin, "-n", systemctlBin, "start", CollectorService).Return([]byte("failed"), cmdErr)
				return cmdErr
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCmd := NewMockcmdRunner(ctrl)
			c := &Collector{os: nil, cmd: mockCmd}
			expectedErr := tt.expects(mockCmd)

			err := c.Start()

			if !errors.Is(err, expectedErr) {
				t.Fatalf("expected error %v, got %v", expectedErr, err)
			}
		})
	}
}

func TestStop(t *testing.T) {
	tests := []struct {
		name    string
		expects func(*MockcmdRunner) error
	}{
		{
			name: "happy path",
			expects: func(cmd *MockcmdRunner) error {
				cmd.EXPECT().Run(sudoBin, "-n", systemctlBin, "stop", CollectorService).Return(nil, nil)
				return nil
			},
		},
		{
			name: "systemctl fails",
			expects: func(cmd *MockcmdRunner) error {
				cmdErr := errors.New("exit status 1")
				cmd.EXPECT().Run(sudoBin, "-n", systemctlBin, "stop", CollectorService).Return([]byte("failed"), cmdErr)
				return cmdErr
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCmd := NewMockcmdRunner(ctrl)
			c := &Collector{os: nil, cmd: mockCmd}
			expectedErr := tt.expects(mockCmd)

			err := c.Stop()

			if !errors.Is(err, expectedErr) {
				t.Fatalf("expected error %v, got %v", expectedErr, err)
			}
		})
	}
}
