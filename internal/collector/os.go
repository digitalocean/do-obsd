package collector

//go:generate go tool mockgen -source=os.go -package=collector -destination=mocks_os_test.go

import (
	"io"
	"os"
)

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
