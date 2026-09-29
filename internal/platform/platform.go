package platform

import (
	"context"
	"errors"
)

var ErrWindowsOnly = errors.New("Windows provisioning is supported only on native Windows 11")

type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}
type PackageManager interface {
	Installed(context.Context, string) (bool, error)
	Install(context.Context, string, string) error
	Uninstall(context.Context, string) error
}
type ExecRunner struct{}
