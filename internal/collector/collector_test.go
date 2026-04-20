package collector

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/mock/gomock"
)

func TestWriteConfig(t *testing.T) {
	t.Setenv("DO_OBSD_COLLECTOR_CONFIG_PATH", filepath.Join(t.TempDir(), "otel", "config.yaml"))
	configData := []byte("service:\n  pipelines: {}\n")
	dest := ResolvedCollectorConfigPath()
	destDir := filepath.Dir(dest)

	tests := []struct {
		name    string
		expects func(*MockosOperator, *MocktempFile) error
	}{
		{
			name: "happy path",
			expects: func(mockOS *MockosOperator, tmp *MocktempFile) error {
				mockOS.EXPECT().CreateTemp(destDir, gomock.Any()).Return(tmp, nil)
				tmp.EXPECT().Name().Return("/tmp/.config-test.yaml").AnyTimes()
				tmp.EXPECT().Write(gomock.Any()).Return(len(configData), nil)
				tmp.EXPECT().Chmod(os.FileMode(0640)).Return(nil)
				tmp.EXPECT().Close().Return(nil)
				mockOS.EXPECT().Rename("/tmp/.config-test.yaml", dest).Return(nil)
				mockOS.EXPECT().Remove("/tmp/.config-test.yaml").Return(nil)
				return nil
			},
		},
		{
			name: "write fails",
			expects: func(mockOS *MockosOperator, tmp *MocktempFile) error {
				writeErr := errors.New("disk full")
				mockOS.EXPECT().CreateTemp(destDir, gomock.Any()).Return(tmp, nil)
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
				mockOS.EXPECT().CreateTemp(destDir, gomock.Any()).Return(tmp, nil)
				tmp.EXPECT().Name().Return("/tmp/.config-test.yaml").AnyTimes()
				tmp.EXPECT().Write(gomock.Any()).Return(len(configData), nil)
				tmp.EXPECT().Chmod(os.FileMode(0640)).Return(nil)
				tmp.EXPECT().Close().Return(nil)
				mockOS.EXPECT().Rename("/tmp/.config-test.yaml", dest).Return(renameErr)
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
