//go:build !windows

package platform

import "context"

func EnsureChezmoi(context.Context, string) error { return ErrWindowsOnly }
