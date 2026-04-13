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
				a.os.EXPECT().Open(bundlePath).Return(io.NopCloser(strings.NewReader("binary")), nil)
				a.os.EXPECT().CreateTemp(filepath.Dir(collectorBin), gomock.Any()).Return(a.tmp, nil)
				a.tmp.EXPECT().Name().Return("/tmp/.do-otelcol-test").AnyTimes()
				a.tmp.EXPECT().Write(gomock.Any()).Return(6, nil)
				a.tmp.EXPECT().Chmod(os.FileMode(0755)).Return(nil)
				a.tmp.EXPECT().Close().Return(nil)
				a.os.EXPECT().Rename("/tmp/.do-otelcol-test", collectorBin).Return(nil)
				a.os.EXPECT().Remove("/tmp/.do-otelcol-test").Return(nil)
				return nil
			},
		},
		{
			name: "bundle not found",
			expects: func(a *args) error {
				a.os.EXPECT().Open(bundlePath).Return(nil, os.ErrNotExist)
				return os.ErrNotExist
			},
		},
		{
			name: "rename fails",
			expects: func(a *args) error {
				renameErr := errors.New("cross-device link")
				a.os.EXPECT().Open(bundlePath).Return(io.NopCloser(strings.NewReader("binary")), nil)
				a.os.EXPECT().CreateTemp(filepath.Dir(collectorBin), gomock.Any()).Return(a.tmp, nil)
				a.tmp.EXPECT().Name().Return("/tmp/.do-otelcol-test").AnyTimes()
				a.tmp.EXPECT().Write(gomock.Any()).Return(6, nil)
				a.tmp.EXPECT().Chmod(os.FileMode(0755)).Return(nil)
				a.tmp.EXPECT().Close().Return(nil)
				a.os.EXPECT().Rename("/tmp/.do-otelcol-test", collectorBin).Return(renameErr)
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

			c := &Collector{os: mockOS}
			expectedErr := tt.expects(&args{mockOS, mockTmp})

			err := c.Install()

			if !errors.Is(err, expectedErr) {
				t.Fatalf("expected error %v, got %v", expectedErr, err)
			}
		})
	}
}

func TestWriteConfig(t *testing.T) {
	configData := []byte("service:\n  pipelines: {}\n")

	tests := []struct {
		name    string
		expects func(*MockosOperator, *MocktempFile) error
	}{
		{
			name: "happy path",
			expects: func(mockOS *MockosOperator, tmp *MocktempFile) error {
				mockOS.EXPECT().CreateTemp(filepath.Dir(collectorConfigPath), gomock.Any()).Return(tmp, nil)
				tmp.EXPECT().Name().Return("/tmp/.config-test.yaml").AnyTimes()
				tmp.EXPECT().Write(gomock.Any()).Return(len(configData), nil)
				tmp.EXPECT().Chmod(os.FileMode(0640)).Return(nil)
				tmp.EXPECT().Close().Return(nil)
				mockOS.EXPECT().Rename("/tmp/.config-test.yaml", collectorConfigPath).Return(nil)
				mockOS.EXPECT().Remove("/tmp/.config-test.yaml").Return(nil)
				return nil
			},
		},
		{
			name: "write fails",
			expects: func(mockOS *MockosOperator, tmp *MocktempFile) error {
				writeErr := errors.New("disk full")
				mockOS.EXPECT().CreateTemp(filepath.Dir(collectorConfigPath), gomock.Any()).Return(tmp, nil)
				tmp.EXPECT().Name().Return("/tmp/.config-test.yaml").AnyTimes()
				tmp.EXPECT().Write(gomock.Any()).Return(0, writeErr)
				tmp.EXPECT().Close().Return(nil)
				mockOS.EXPECT().Remove("/tmp/.config-test.yaml").Return(nil)
				return writeErr
			},
		},
		{
			name: "rename fails",
			expects: func(mockOS *MockosOperator, tmp *MocktempFile) error {
				renameErr := errors.New("cross-device link")
				mockOS.EXPECT().CreateTemp(filepath.Dir(collectorConfigPath), gomock.Any()).Return(tmp, nil)
				tmp.EXPECT().Name().Return("/tmp/.config-test.yaml").AnyTimes()
				tmp.EXPECT().Write(gomock.Any()).Return(len(configData), nil)
				tmp.EXPECT().Chmod(os.FileMode(0640)).Return(nil)
				tmp.EXPECT().Close().Return(nil)
				mockOS.EXPECT().Rename("/tmp/.config-test.yaml", collectorConfigPath).Return(renameErr)
				mockOS.EXPECT().Remove("/tmp/.config-test.yaml").Return(nil)
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
			c := &Collector{os: mockOS}
			expectedErr := tt.expects(mockOS, mockTmp)

			err := c.WriteConfig(configData)

			if !errors.Is(err, expectedErr) {
				t.Fatalf("expected error %v, got %v", expectedErr, err)
			}
		})
	}
}
