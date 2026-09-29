//go:build !windows

package platform

import "context"

type WinGet struct{ Runner Runner }

func (WinGet) Installed(context.Context, string) (bool, error) { return false, ErrWindowsOnly }
func (WinGet) Install(context.Context, string, string) error   { return ErrWindowsOnly }
func (WinGet) Uninstall(context.Context, string) error         { return ErrWindowsOnly }
