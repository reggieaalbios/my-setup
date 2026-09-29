package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/reggieaalbios/my-setup/internal/catalog"
)

type Observed struct {
	Installed bool   `json:"installed"`
	Owned     bool   `json:"owned"`
	Drifted   bool   `json:"drifted"`
	Scope     string `json:"scope,omitempty"`
}
type ManagedFile struct {
	Component string `json:"component"`
	Path      string `json:"path"`
	Hash      string `json:"hash"`
}
type PackageOwnership struct {
	ID          string `json:"id"`
	PreExisting bool   `json:"pre_existing"`
	Scope       string `json:"scope,omitempty"`
}
type Ledger struct {
	SchemaVersion int                         `json:"schema_version"`
	EngineVersion string                      `json:"engine_version"`
	SourceCommit  string                      `json:"source_commit,omitempty"`
	Selections    map[string]catalog.Choice   `json:"selections"`
	Packages      map[string]PackageOwnership `json:"packages"`
	Files         map[string]ManagedFile      `json:"files"`
	UpdatedAt     time.Time                   `json:"updated_at"`
}
type Transaction struct {
	ID         string                    `json:"id"`
	Phase      string                    `json:"phase"`
	StartedAt  time.Time                 `json:"started_at"`
	Selections map[string]catalog.Choice `json:"selections,omitempty"`
	Actions    []TransactionAction       `json:"actions"`
}
type TransactionAction struct {
	Component string `json:"component"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

type Store struct{ Root string }

func DefaultRoot() (string, error) {
	if runtime.GOOS == "windows" {
		if root := os.Getenv("LOCALAPPDATA"); root != "" {
			return filepath.Join(root, "mysetup"), nil
		}
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "mysetup"), nil
}
func (s Store) Load() (Ledger, error) {
	data, err := os.ReadFile(filepath.Join(s.Root, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return NewLedger(), nil
	}
	if err != nil {
		return Ledger{}, err
	}
	var state Ledger
	if err := json.Unmarshal(data, &state); err != nil {
		return Ledger{}, err
	}
	return state, nil
}
func NewLedger() Ledger {
	return Ledger{SchemaVersion: 1, Selections: map[string]catalog.Choice{}, Packages: map[string]PackageOwnership{}, Files: map[string]ManagedFile{}}
}
func (s Store) Save(value any, name string) error {
	if err := os.MkdirAll(s.Root, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(s.Root, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
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
	return os.Rename(tmpName, filepath.Join(s.Root, name))
}
func (s Store) AppendLog(tx Transaction) error {
	dir := filepath.Join(s.Root, "transactions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, _ := json.Marshal(tx)
	f, err := os.OpenFile(filepath.Join(dir, tx.ID+".jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}
func HashFile(name string) (string, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
func NewTransaction() Transaction {
	now := time.Now().UTC()
	return Transaction{ID: fmt.Sprintf("%d", now.UnixNano()), Phase: "planned", StartedAt: now, Actions: []TransactionAction{}}
}
