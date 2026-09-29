//go:build windows

package platform

import (
	"context"
	"fmt"
	"strings"
)

type WinGet struct{ Runner Runner }

func (w WinGet) run(ctx context.Context, args ...string) error {
	out, err := w.Runner.Run(ctx, "winget", args...)
	if err != nil {
		return fmt.Errorf("winget %v: %w: %s", args, err, out)
	}
	return nil
}
func (w WinGet) Installed(ctx context.Context, id string) (bool, error) {
	out, err := w.Runner.Run(ctx, "winget", "list", "--id", id, "-e", "--source", "winget", "--accept-source-agreements", "--disable-interactivity")
	if err != nil {
		return false, nil
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(id)), nil
}
func (w WinGet) Install(ctx context.Context, id, scope string) error {
	args := []string{"install", "--id", id, "-e", "--source", "winget", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}
	if scope != "" {
		args = append(args, "--scope", scope)
	}
	return w.run(ctx, args...)
}
func (w WinGet) Uninstall(ctx context.Context, id string) error {
	return w.run(ctx, "uninstall", "--id", id, "-e", "--source", "winget", "--disable-interactivity")
}
