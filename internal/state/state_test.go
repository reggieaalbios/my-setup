package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicSaveAndLoad(t *testing.T) {
	store := Store{Root: t.TempDir()}
	ledger := NewLedger()
	ledger.EngineVersion = "0.1.0"
	if err := store.Save(ledger, "state.json"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.EngineVersion != "0.1.0" {
		t.Fatalf("version=%q", loaded.EngineVersion)
	}
	matches, err := filepath.Glob(filepath.Join(store.Root, ".state-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files=%v err=%v", matches, err)
	}
	if info, err := os.Stat(filepath.Join(store.Root, "state.json")); err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatalf("unsafe permissions or stat error: %v %v", info, err)
	}
}
