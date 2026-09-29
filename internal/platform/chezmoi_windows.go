//go:build windows

package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const chezmoiURL = "https://github.com/twpayne/chezmoi/releases/download/v2.73.0/chezmoi-windows-amd64.exe"
const chezmoiSHA256 = "8cb8353d6a20719d97e3106ae652724ecba3795ef08c761d9d8ce4f624441a47"

func EnsureChezmoi(ctx context.Context, destination string) error {
	if sum, err := fileSHA256(destination); err == nil && sum == chezmoiSHA256 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, chezmoiURL, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download chezmoi: %s", response.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".chezmoi-*.exe")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(response.Body, 128<<20)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); actual != chezmoiSHA256 {
		return fmt.Errorf("chezmoi checksum mismatch: got %s", actual)
	}
	return os.Rename(name, destination)
}
func fileSHA256(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
