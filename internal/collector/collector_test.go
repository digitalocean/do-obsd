package collector

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.os == nil || c.cmd == nil || c.http == nil {
		t.Fatal("New() returned Collector with nil dependencies")
	}
}

func TestInstall(t *testing.T) {
	okBody := "binary"

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockHTTP := NewMockhttpDoer(ctrl)
		mockOS := NewMockosOperator(ctrl)
		mockTmp := NewMocktempFile(ctrl)

		mockHTTP.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(okBody)),
		}, nil)
		mockOS.EXPECT().CreateTemp(filepath.Dir(CollectorBin), gomock.Any()).Return(mockTmp, nil)
		mockTmp.EXPECT().Name().Return("/tmp/.do-otelcol-test").AnyTimes()
		mockTmp.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) { return len(p), nil }).AnyTimes()
		mockTmp.EXPECT().Chmod(os.FileMode(0o755)).Return(nil)
		mockTmp.EXPECT().Close().Return(nil)
		mockOS.EXPECT().Rename("/tmp/.do-otelcol-test", CollectorBin).Return(nil)
		mockOS.EXPECT().Remove("/tmp/.do-otelcol-test").Return(nil)

		c := &Collector{os: mockOS, http: mockHTTP}
		if err := c.Install(); err != nil {
			t.Fatalf("Install: %v", err)
		}
	})

	t.Run("http error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockHTTP := NewMockhttpDoer(ctrl)
		mockHTTP.EXPECT().Do(gomock.Any()).Return(nil, errors.New("network down"))

		c := &Collector{http: mockHTTP}
		err := c.Install()
		if err == nil || !strings.Contains(err.Error(), "download collector") {
			t.Fatalf("expected download error, got %v", err)
		}
	})

	t.Run("non-200", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockHTTP := NewMockhttpDoer(ctrl)
		mockHTTP.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusNotFound,
			Status:     http.StatusText(http.StatusNotFound),
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil)

		c := &Collector{http: mockHTTP}
		err := c.Install()
		if err == nil || !strings.Contains(err.Error(), "404") {
			t.Fatalf("expected HTTP error, got %v", err)
		}
	})

	t.Run("non-200 internal server error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockHTTP := NewMockhttpDoer(ctrl)
		mockHTTP.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusInternalServerError,
			Status:     http.StatusText(http.StatusInternalServerError),
			Body:       io.NopCloser(strings.NewReader("error body")),
		}, nil)

		c := &Collector{http: mockHTTP}
		err := c.Install()
		if err == nil || !strings.Contains(err.Error(), "500") {
			t.Fatalf("expected HTTP 500 in error, got %v", err)
		}
	})

	t.Run("create temp fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockHTTP := NewMockhttpDoer(ctrl)
		mockOS := NewMockosOperator(ctrl)
		createErr := errors.New("no space left on device")

		mockHTTP.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(okBody)),
		}, nil)
		mockOS.EXPECT().CreateTemp(filepath.Dir(CollectorBin), gomock.Any()).Return(nil, createErr)

		c := &Collector{os: mockOS, http: mockHTTP}
		err := c.Install()
		if !errors.Is(err, createErr) {
			t.Fatalf("expected create temp error, got %v", err)
		}
	})

	t.Run("write copy fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockHTTP := NewMockhttpDoer(ctrl)
		mockOS := NewMockosOperator(ctrl)
		mockTmp := NewMocktempFile(ctrl)
		writeErr := errors.New("write: input/output error")

		mockHTTP.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(okBody)),
		}, nil)
		mockOS.EXPECT().CreateTemp(filepath.Dir(CollectorBin), gomock.Any()).Return(mockTmp, nil)
		mockTmp.EXPECT().Name().Return("/tmp/.do-otelcol-test").AnyTimes()
		mockTmp.EXPECT().Write(gomock.Any()).Return(0, writeErr)
		mockTmp.EXPECT().Close().Return(nil)
		mockOS.EXPECT().Remove("/tmp/.do-otelcol-test").Return(nil)

		c := &Collector{os: mockOS, http: mockHTTP}
		err := c.Install()
		if err == nil || !strings.Contains(err.Error(), "write downloaded binary") {
			t.Fatalf("expected write error, got %v", err)
		}
		if !errors.Is(err, writeErr) {
			t.Fatalf("expected wrapped writeErr, got %v", err)
		}
	})

	t.Run("chmod fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockHTTP := NewMockhttpDoer(ctrl)
		mockOS := NewMockosOperator(ctrl)
		mockTmp := NewMocktempFile(ctrl)
		chmodErr := errors.New("operation not permitted")

		mockHTTP.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(okBody)),
		}, nil)
		mockOS.EXPECT().CreateTemp(filepath.Dir(CollectorBin), gomock.Any()).Return(mockTmp, nil)
		mockTmp.EXPECT().Name().Return("/tmp/.do-otelcol-test").AnyTimes()
		mockTmp.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) { return len(p), nil }).AnyTimes()
		mockTmp.EXPECT().Chmod(os.FileMode(0o755)).Return(chmodErr)
		mockTmp.EXPECT().Close().Return(nil)
		mockOS.EXPECT().Remove("/tmp/.do-otelcol-test").Return(nil)

		c := &Collector{os: mockOS, http: mockHTTP}
		err := c.Install()
		if !errors.Is(err, chmodErr) {
			t.Fatalf("expected chmod error, got %v", err)
		}
	})

	t.Run("close temp fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockHTTP := NewMockhttpDoer(ctrl)
		mockOS := NewMockosOperator(ctrl)
		mockTmp := NewMocktempFile(ctrl)
		closeErr := errors.New("bad file descriptor")

		mockHTTP.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(okBody)),
		}, nil)
		mockOS.EXPECT().CreateTemp(filepath.Dir(CollectorBin), gomock.Any()).Return(mockTmp, nil)
		mockTmp.EXPECT().Name().Return("/tmp/.do-otelcol-test").AnyTimes()
		mockTmp.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) { return len(p), nil }).AnyTimes()
		mockTmp.EXPECT().Chmod(os.FileMode(0o755)).Return(nil)
		mockTmp.EXPECT().Close().Return(closeErr)
		mockOS.EXPECT().Remove("/tmp/.do-otelcol-test").Return(nil)

		c := &Collector{os: mockOS, http: mockHTTP}
		err := c.Install()
		if !errors.Is(err, closeErr) {
			t.Fatalf("expected close error, got %v", err)
		}
	})

	t.Run("rename fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockHTTP := NewMockhttpDoer(ctrl)
		mockOS := NewMockosOperator(ctrl)
		mockTmp := NewMocktempFile(ctrl)
		renameErr := errors.New("cross-device link")

		mockHTTP.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(okBody)),
		}, nil)
		mockOS.EXPECT().CreateTemp(filepath.Dir(CollectorBin), gomock.Any()).Return(mockTmp, nil)
		mockTmp.EXPECT().Name().Return("/tmp/.do-otelcol-test").AnyTimes()
		mockTmp.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (int, error) { return len(p), nil }).AnyTimes()
		mockTmp.EXPECT().Chmod(os.FileMode(0o755)).Return(nil)
		mockTmp.EXPECT().Close().Return(nil)
		mockOS.EXPECT().Rename("/tmp/.do-otelcol-test", CollectorBin).Return(renameErr)
		mockOS.EXPECT().Remove("/tmp/.do-otelcol-test").Return(nil)

		c := &Collector{os: mockOS, http: mockHTTP}
		err := c.Install()
		if !errors.Is(err, renameErr) {
			t.Fatalf("expected rename err, got %v", err)
		}
	})
}

func TestStart(t *testing.T) {
	tests := []struct {
		name    string
		expects func(*MockcmdRunner) error
	}{
		{
			name: "happy path",
			expects: func(cmd *MockcmdRunner) error {
				cmd.EXPECT().Run("systemctl", "start", CollectorService).Return(nil, nil)
				return nil
			},
		},
		{
			name: "systemctl fails",
			expects: func(cmd *MockcmdRunner) error {
				cmdErr := errors.New("exit status 1")
				cmd.EXPECT().Run("systemctl", "start", CollectorService).Return([]byte("failed"), cmdErr)
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
				cmd.EXPECT().Run("systemctl", "stop", CollectorService).Return(nil, nil)
				return nil
			},
		},
		{
			name: "systemctl fails",
			expects: func(cmd *MockcmdRunner) error {
				cmdErr := errors.New("exit status 1")
				cmd.EXPECT().Run("systemctl", "stop", CollectorService).Return([]byte("failed"), cmdErr)
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
