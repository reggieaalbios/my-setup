package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/reggieaalbios/my-setup/internal/catalog"
	"github.com/reggieaalbios/my-setup/internal/state"
)

type failingRunner struct{ target string }

func (f failingRunner) Run(context.Context, string, ...string) ([]byte, error) {
	if err := os.WriteFile(f.target, []byte("new"), 0600); err != nil {
		return nil, err
	}
	return []byte("injected"), errors.New("failure")
}
func TestConfigFailureRestoresOriginal(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, "managed.txt")
	if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	source := fstest.MapFS{"chezmoi/managed.txt": {Data: []byte("new")}}
	manager := Chezmoi{Runner: failingRunner{target: target}, Binary: "chezmoi", Home: home, Source: source}
	component := catalog.Component{ID: "x", Config: &catalog.Config{Targets: []string{"managed.txt"}}}
	if _, err := manager.Apply(context.Background(), component, t.TempDir()); err == nil {
		t.Fatal("expected failure")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("restored=%q", data)
	}
}
func TestRemovePreservesDrift(t *testing.T) {
	name := filepath.Join(t.TempDir(), "managed.txt")
	if err := os.WriteFile(name, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	manager := Chezmoi{}
	if err := manager.Remove(context.Background(), []state.ManagedFile{{Path: name, Hash: "different"}}, false); err == nil {
		t.Fatal("expected drift protection")
	}
	if _, err := os.Stat(name); err != nil {
		t.Fatal("drifted file removed")
	}
}
