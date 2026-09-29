package source

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/reggieaalbios/my-setup/internal/catalog"
)

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type GitHub struct {
	Owner, Repo string
	Client      *http.Client
}
type commitResponse struct {
	SHA string `json:"sha"`
}

func (g GitHub) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return http.DefaultClient
}
func (g GitHub) ResolveMain(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/main", g.Owner, g.Repo)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	response, err := g.client().Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve main: %s", response.Status)
	}
	var result commitResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", err
	}
	if !commitPattern.MatchString(result.SHA) {
		return "", fmt.Errorf("invalid commit SHA")
	}
	return result.SHA, nil
}
func (g GitHub) Stage(ctx context.Context, commit, root, engine string) (string, catalog.Catalog, error) {
	if !commitPattern.MatchString(commit) {
		return "", catalog.Catalog{}, fmt.Errorf("invalid commit SHA")
	}
	staging, err := os.MkdirTemp(root, "snapshot-")
	if err != nil {
		return "", catalog.Catalog{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(staging)
		}
	}()
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", g.Owner, g.Repo, commit)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	response, err := g.client().Do(req)
	if err != nil {
		return "", catalog.Catalog{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", catalog.Catalog{}, fmt.Errorf("download snapshot: %s", response.Status)
	}
	if err := extract(response.Body, staging); err != nil {
		return "", catalog.Catalog{}, err
	}
	entries, err := os.ReadDir(staging)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		return "", catalog.Catalog{}, fmt.Errorf("snapshot archive has unexpected layout")
	}
	checkout := filepath.Join(staging, entries[0].Name())
	contentRoot := filepath.Join(checkout, "content")
	snapshotFS := os.DirFS(contentRoot)
	if err := catalog.ValidateIntegrity(snapshotFS); err != nil {
		return "", catalog.Catalog{}, err
	}
	loaded, err := catalog.Load(snapshotFS, engine)
	if err != nil {
		return "", catalog.Catalog{}, err
	}
	cleanup = false
	return checkout, loaded, nil
}
func Activate(checkout, stateRoot, commit string) error {
	snapshots := filepath.Join(stateRoot, "snapshots")
	if err := os.MkdirAll(snapshots, 0700); err != nil {
		return err
	}
	destination := filepath.Join(snapshots, commit)
	if _, err := os.Stat(destination); os.IsNotExist(err) {
		if err := os.Rename(checkout, destination); err != nil {
			return err
		}
		_ = os.Remove(filepath.Dir(checkout))
	}
	pointer := filepath.Join(snapshots, "active")
	tmp := pointer + ".tmp"
	if err := os.WriteFile(tmp, []byte(commit+"\n"), 0600); err != nil {
		return err
	}
	return replaceFile(tmp, pointer)
}
func ActiveFS(stateRoot string) (fs.FS, string, error) {
	data, err := os.ReadFile(filepath.Join(stateRoot, "snapshots", "active"))
	if os.IsNotExist(err) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	commit := strings.TrimSpace(string(data))
	if !commitPattern.MatchString(commit) {
		return nil, "", fmt.Errorf("invalid active snapshot pointer")
	}
	root := filepath.Join(stateRoot, "snapshots", commit, "content")
	if _, err := os.Stat(root); err != nil {
		return nil, "", err
	}
	return os.DirFS(root), commit, nil
}
func extract(reader io.Reader, destination string) error {
	gz, err := gzip.NewReader(io.LimitReader(reader, 256<<20))
	if err != nil {
		return err
	}
	defer gz.Close()
	archive := tar.NewReader(gz)
	var total int64
	files := 0
	for {
		header, err := archive.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe archive path %q", header.Name)
		}
		target := filepath.Join(destination, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0700); err != nil {
				return err
			}
		case tar.TypeReg:
			files++
			total += header.Size
			if files > 10000 || total > 512<<20 {
				return fmt.Errorf("snapshot exceeds extraction limits")
			}
			if header.Size > 32<<20 {
				return fmt.Errorf("archive file too large: %s", header.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, io.LimitReader(archive, header.Size))
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			continue
		}
	}
}
