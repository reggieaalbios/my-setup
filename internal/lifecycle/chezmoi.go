package lifecycle

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/reggieaalbios/my-setup/content"
	"github.com/reggieaalbios/my-setup/internal/catalog"
	"github.com/reggieaalbios/my-setup/internal/platform"
	"github.com/reggieaalbios/my-setup/internal/state"
)

type Chezmoi struct {
	Runner       platform.Runner
	Binary, Home string
	Source       fs.FS
}

func (c Chezmoi) Apply(ctx context.Context, component catalog.Component, txRoot string) ([]state.ManagedFile, error) {
	if component.Config == nil {
		return nil, nil
	}
	sourceRoot := filepath.Join(txRoot, "source")
	backupRoot := filepath.Join(txRoot, "originals")
	sourceFS := c.Source
	if sourceFS == nil {
		sourceFS = content.FS
	}
	if err := extractSource(sourceFS, sourceRoot); err != nil {
		return nil, err
	}
	for _, target := range component.Config.Targets {
		dst, err := safeJoin(c.Home, target)
		if err != nil {
			return nil, err
		}
		if data, err := os.ReadFile(dst); err == nil {
			backup := filepath.Join(backupRoot, filepath.FromSlash(target))
			if err := os.MkdirAll(filepath.Dir(backup), 0700); err != nil {
				return nil, err
			}
			if err := os.WriteFile(backup, data, 0600); err != nil {
				return nil, err
			}
		}
	}
	args := []string{"apply", "--source", sourceRoot, "--destination", c.Home, "--force", "--no-tty"}
	args = append(args, component.Config.Targets...)
	out, err := c.Runner.Run(ctx, c.Binary, args...)
	if err != nil {
		_ = restore(component.Config.Targets, c.Home, backupRoot)
		return nil, fmt.Errorf("chezmoi apply: %w: %s", err, out)
	}
	files := []state.ManagedFile{}
	for _, target := range component.Config.Targets {
		dst, err := safeJoin(c.Home, target)
		if err != nil {
			return nil, err
		}
		hash, err := state.HashFile(dst)
		if err != nil {
			_ = restore(component.Config.Targets, c.Home, backupRoot)
			return nil, err
		}
		files = append(files, state.ManagedFile{Component: component.ID, Path: dst, Hash: hash})
	}
	return files, nil
}
func (c Chezmoi) Remove(_ context.Context, files []state.ManagedFile, allowDrift bool) error {
	for _, file := range files {
		if c.Home != "" {
			relative, err := filepath.Rel(c.Home, file.Path)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return fmt.Errorf("refusing managed path outside home: %s", file.Path)
			}
		}
		hash, err := state.HashFile(file.Path)
		if err != nil && os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if hash != file.Hash && !allowDrift {
			return fmt.Errorf("preserving drifted config %s; confirm with --allow-drift", file.Path)
		}
		if err := os.Remove(file.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func safeJoin(home, target string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(target))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.VolumeName(clean) != "" {
		return "", fmt.Errorf("unsafe config target %q", target)
	}
	return filepath.Join(home, clean), nil
}
func extractSource(sourceFS fs.FS, root string) error {
	return fs.WalkDir(sourceFS, "chezmoi", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative := strings.TrimPrefix(name, "chezmoi/")
		if relative == "chezmoi" || relative == "" {
			return os.MkdirAll(root, 0700)
		}
		dst := filepath.Join(root, filepath.FromSlash(relative))
		if d.IsDir() {
			return os.MkdirAll(dst, 0700)
		}
		data, err := fs.ReadFile(sourceFS, name)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0600)
	})
}
func restore(targets []string, home, backup string) error {
	for _, target := range targets {
		src := filepath.Join(backup, filepath.FromSlash(target))
		data, err := os.ReadFile(src)
		dst := filepath.Join(home, filepath.FromSlash(target))
		if os.IsNotExist(err) {
			_ = os.Remove(dst)
			continue
		}
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0600); err != nil {
			return err
		}
	}
	return nil
}
