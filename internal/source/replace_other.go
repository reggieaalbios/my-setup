//go:build !windows

package source

import "os"

func replaceFile(source, destination string) error { return os.Rename(source, destination) }
