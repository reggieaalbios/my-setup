package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/reggieaalbios/my-setup/internal/catalog"
	"github.com/reggieaalbios/my-setup/internal/planner"
	"github.com/reggieaalbios/my-setup/internal/platform"
	"github.com/reggieaalbios/my-setup/internal/state"
)

type ConfigManager interface {
	Apply(context.Context, catalog.Component, string) ([]state.ManagedFile, error)
	Remove(context.Context, []state.ManagedFile, bool) error
}
type Engine struct {
	Catalog  catalog.Catalog
	Packages platform.PackageManager
	Configs  ConfigManager
	Store    state.Store
	Version  string
}

func (e Engine) Observe(ctx context.Context) (map[string]state.Observed, error) {
	ledger, err := e.Store.Load()
	if err != nil {
		return nil, err
	}
	out := map[string]state.Observed{}
	for id, component := range e.Catalog.Components {
		obs := state.Observed{}
		if component.Package != nil {
			installed, err := e.Packages.Installed(ctx, component.Package.ID)
			if err != nil {
				return nil, err
			}
			obs.Installed = installed
			if own, ok := ledger.Packages[id]; ok {
				obs.Owned = !own.PreExisting
				obs.Scope = own.Scope
			}
		}
		if component.Config != nil {
			for _, f := range ledger.Files {
				if f.Component == id {
					hash, err := state.HashFile(f.Path)
					if err != nil || hash != f.Hash {
						obs.Drifted = true
					}
				}
			}
		}
		out[id] = obs
	}
	return out, nil
}

func (e Engine) Apply(ctx context.Context, plan planner.Plan, selections map[string]catalog.Choice) error {
	ledger, err := e.Store.Load()
	if err != nil {
		return err
	}
	tx := state.NewTransaction()
	tx.Selections = selections
	tx.Phase = "staging"
	if err := e.checkpoint(tx); err != nil {
		return err
	}
	txRoot := filepath.Join(e.Store.Root, "staging", tx.ID)
	if err := os.MkdirAll(txRoot, 0700); err != nil {
		return err
	}
	success := false
	defer func() {
		if success {
			_ = os.RemoveAll(txRoot)
		}
	}()
	for _, action := range plan.Actions {
		component := e.Catalog.Components[action.Component]
		record := state.TransactionAction{Component: action.Component, Kind: string(action.Kind), Status: "running"}
		tx.Actions = append(tx.Actions, record)
		switch action.Kind {
		case planner.InstallPackage:
			tx.Phase = "packages-installing"
			if err := e.checkpoint(tx); err != nil {
				return err
			}
			pre, err := e.Packages.Installed(ctx, action.PackageID)
			if err != nil {
				return e.fail(tx, err)
			}
			if !pre {
				if err := e.Packages.Install(ctx, action.PackageID, component.Package.Scope); err != nil {
					return e.fail(tx, err)
				}
				installed, err := e.Packages.Installed(ctx, action.PackageID)
				if err != nil {
					return e.fail(tx, err)
				}
				if !installed {
					return e.fail(tx, fmt.Errorf("package %s was not detected after installation", action.PackageID))
				}
			}
			ledger.Packages[action.Component] = state.PackageOwnership{ID: action.PackageID, PreExisting: pre, Scope: component.Package.Scope}
			ledger.EngineVersion = e.Version
			ledger.Selections = selections
			ledger.UpdatedAt = time.Now().UTC()
			if err := e.Store.Save(ledger, "state.json"); err != nil {
				return e.fail(tx, err)
			}
		case planner.ApplyConfig:
			tx.Phase = "configs-applying"
			if err := e.checkpoint(tx); err != nil {
				return err
			}
			files, err := e.Configs.Apply(ctx, component, txRoot)
			if err != nil {
				return e.fail(tx, err)
			}
			for _, file := range files {
				ledger.Files[file.Path] = file
			}
			ledger.UpdatedAt = time.Now().UTC()
			if err := e.Store.Save(ledger, "state.json"); err != nil {
				return e.fail(tx, err)
			}
		}
		tx.Actions[len(tx.Actions)-1].Status = "complete"
		if err := e.checkpoint(tx); err != nil {
			return err
		}
	}
	tx.Phase = "committed"
	ledger.EngineVersion = e.Version
	ledger.Selections = selections
	ledger.UpdatedAt = time.Now().UTC()
	if err := e.Store.Save(ledger, "state.json"); err != nil {
		return e.fail(tx, err)
	}
	if err := e.checkpoint(tx); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(e.Store.Root, "incomplete.json"))
	success = true
	return nil
}
func (e Engine) Remove(ctx context.Context, ids []string, allowDrift bool) error {
	ledger, err := e.Store.Load()
	if err != nil {
		return err
	}
	selected := map[string]bool{}
	for _, id := range ids {
		if _, ok := e.Catalog.Components[id]; !ok {
			return fmt.Errorf("unknown component %q", id)
		}
		selected[id] = true
	}
	for _, id := range ids {
		if own, ok := ledger.Packages[id]; ok && !own.PreExisting {
			for other, choice := range ledger.Selections {
				if other == id || selected[other] || !choice.Package {
					continue
				}
				for _, dep := range e.Catalog.Components[other].Required {
					if dep == id {
						return fmt.Errorf("cannot remove %s: required by %s", id, other)
					}
				}
			}
		}
	}
	for _, id := range ids {
		component, ok := e.Catalog.Components[id]
		if !ok {
			return fmt.Errorf("unknown component %q", id)
		}
		files := []state.ManagedFile{}
		for _, file := range ledger.Files {
			if file.Component == id {
				files = append(files, file)
			}
		}
		if err := e.Configs.Remove(ctx, files, allowDrift); err != nil {
			return err
		}
		for _, file := range files {
			delete(ledger.Files, file.Path)
		}
		if own, ok := ledger.Packages[id]; ok && !own.PreExisting {
			if component.Package != nil {
				if err := e.Packages.Uninstall(ctx, component.Package.ID); err != nil {
					return err
				}
			}
			delete(ledger.Packages, id)
		}
		delete(ledger.Selections, id)
	}
	ledger.UpdatedAt = time.Now().UTC()
	return e.Store.Save(ledger, "state.json")
}
func (e Engine) Repair(ctx context.Context) error {
	var tx state.Transaction
	data, err := os.ReadFile(filepath.Join(e.Store.Root, "incomplete.json"))
	if err != nil {
		return fmt.Errorf("no interrupted transaction: %w", err)
	}
	if err := json.Unmarshal(data, &tx); err != nil {
		return fmt.Errorf("invalid interrupted transaction: %w", err)
	}
	if len(tx.Selections) == 0 {
		return fmt.Errorf("transaction %s stopped in phase %s but has no recoverable selections", tx.ID, tx.Phase)
	}
	observed, err := e.Observe(ctx)
	if err != nil {
		return err
	}
	plan, err := planner.Build(e.Catalog, tx.Selections, observed, "Repair")
	if err != nil {
		return err
	}
	return e.Apply(ctx, plan, tx.Selections)
}
func (e Engine) checkpoint(tx state.Transaction) error {
	if err := e.Store.Save(tx, "incomplete.json"); err != nil {
		return err
	}
	return e.Store.AppendLog(tx)
}
func (e Engine) fail(tx state.Transaction, err error) error {
	tx.Phase = "failed"
	if len(tx.Actions) > 0 {
		tx.Actions[len(tx.Actions)-1].Status = "failed"
		tx.Actions[len(tx.Actions)-1].Error = err.Error()
	}
	_ = e.checkpoint(tx)
	return err
}
