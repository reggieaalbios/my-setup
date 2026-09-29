package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/reggieaalbios/my-setup/content"
	"github.com/reggieaalbios/my-setup/internal/capture"
	"github.com/reggieaalbios/my-setup/internal/catalog"
	"github.com/reggieaalbios/my-setup/internal/lifecycle"
	"github.com/reggieaalbios/my-setup/internal/planner"
	"github.com/reggieaalbios/my-setup/internal/platform"
	"github.com/reggieaalbios/my-setup/internal/source"
	"github.com/reggieaalbios/my-setup/internal/state"
	"github.com/reggieaalbios/my-setup/internal/tui"
	"github.com/spf13/cobra"
)

type options struct {
	profile, packages, configs, source string
	yes, json, allowDrift              bool
}
type runtimeApp struct {
	version string
	catalog catalog.Catalog
	engine  lifecycle.Engine
	store   state.Store
	out     io.Writer
}

func NewCommand(version string) *cobra.Command {
	root := &cobra.Command{Use: "mysetup", Short: "Own and reproduce a native Windows 11 setup", SilenceUsage: true}
	run := func(cmd *cobra.Command) (*runtimeApp, error) {
		c, err := catalog.Load(content.FS, version)
		if err != nil {
			return nil, err
		}
		rootPath, err := state.DefaultRoot()
		if err != nil {
			return nil, err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		store := state.Store{Root: rootPath}
		var catalogFS fs.FS = content.FS
		activeFS, _, activeErr := source.ActiveFS(rootPath)
		if activeErr != nil {
			return nil, activeErr
		}
		if activeFS != nil {
			catalogFS = activeFS
			c, err = catalog.Load(catalogFS, version)
			if err != nil {
				return nil, err
			}
		}
		runner := platform.ExecRunner{}
		engine := lifecycle.Engine{Catalog: c, Packages: platform.WinGet{Runner: runner}, Configs: lifecycle.Chezmoi{Runner: runner, Binary: filepath.Join(rootPath, "tools", "chezmoi.exe"), Home: home, Source: catalogFS}, Store: store, Version: version}
		return &runtimeApp{version: version, catalog: c, engine: engine, store: store, out: cmd.OutOrStdout()}, nil
	}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if !terminal(os.Stdout) {
			return cmd.Help()
		}
		a, err := run(cmd)
		if err != nil {
			return err
		}
		result, err := tea.NewProgram(tui.New(a.catalog)).Run()
		if err != nil {
			return err
		}
		model := result.(tui.Model)
		if runtime.GOOS != "windows" || !model.Submit {
			return nil
		}
		removeIDs := []string{}
		for id, remove := range model.Remove {
			if remove {
				removeIDs = append(removeIDs, id)
			}
		}
		sort.Strings(removeIDs)
		if len(removeIDs) > 0 {
			if !confirm(cmd.InOrStdin(), a.out, "Remove selected owned items?") {
				return nil
			}
			if err := a.engine.Remove(cmd.Context(), removeIDs, false); err != nil {
				return err
			}
		}
		observed, err := a.engine.Observe(cmd.Context())
		if err != nil {
			return err
		}
		plan, err := planner.Build(a.catalog, model.Choices, observed, model.Profile)
		if err != nil {
			return err
		}
		if err := printValue(a.out, plan, false); err != nil {
			return err
		}
		if !confirm(cmd.InOrStdin(), a.out, "Apply this plan?") {
			return nil
		}
		if err := platform.EnsureChezmoi(cmd.Context(), filepath.Join(a.store.Root, "tools", "chezmoi.exe")); err != nil {
			return err
		}
		return a.engine.Apply(cmd.Context(), plan, model.Choices)
	}

	var opts options
	addSelectionFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&opts.profile, "profile", "", "profile: Custom, Minimal, Developer, or Full")
		cmd.Flags().StringVar(&opts.packages, "packages", "", "comma-separated package component IDs")
		cmd.Flags().StringVar(&opts.configs, "configs", "", "comma-separated config component IDs")
		cmd.Flags().BoolVar(&opts.yes, "yes", false, "apply without confirmation")
		cmd.Flags().BoolVar(&opts.json, "json", false, "write stable JSON output")
		cmd.Flags().StringVar(&opts.source, "source", "", "source snapshot or commit")
	}
	planCmd := &cobra.Command{Use: "plan", Short: "Produce a no-write plan", RunE: func(cmd *cobra.Command, args []string) error {
		a, err := run(cmd)
		if err != nil {
			return err
		}
		selections, profile, err := resolveSelections(a.catalog, a.store, opts)
		if err != nil {
			return err
		}
		observed, err := a.engine.Observe(cmd.Context())
		if err != nil && runtime.GOOS != "windows" {
			observed = map[string]state.Observed{}
		} else if err != nil {
			return err
		}
		plan, err := planner.Build(a.catalog, selections, observed, profile)
		if err != nil {
			return err
		}
		return printValue(a.out, plan, opts.json)
	}}
	addSelectionFlags(planCmd)
	root.AddCommand(planCmd)
	applyCmd := &cobra.Command{Use: "apply", Short: "Apply current or supplied selections", RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "windows" {
			return platform.ErrWindowsOnly
		}
		a, err := run(cmd)
		if err != nil {
			return err
		}
		selections, profile, err := resolveSelections(a.catalog, a.store, opts)
		if err != nil {
			return err
		}
		observed, err := a.engine.Observe(cmd.Context())
		if err != nil {
			return err
		}
		plan, err := planner.Build(a.catalog, selections, observed, profile)
		if err != nil {
			return err
		}
		if err := printValue(a.out, plan, opts.json); err != nil {
			return err
		}
		if !opts.yes && !confirm(cmd.InOrStdin(), a.out, "Apply this plan?") {
			return fmt.Errorf("cancelled")
		}
		if err := platform.EnsureChezmoi(cmd.Context(), filepath.Join(a.store.Root, "tools", "chezmoi.exe")); err != nil {
			return fmt.Errorf("prepare private chezmoi: %w", err)
		}
		return a.engine.Apply(cmd.Context(), plan, selections)
	}}
	addSelectionFlags(applyCmd)
	root.AddCommand(applyCmd)
	statusCmd := &cobra.Command{Use: "status", Short: "Show selections, ownership, drift, and source commit", RunE: func(cmd *cobra.Command, args []string) error {
		a, err := run(cmd)
		if err != nil {
			return err
		}
		ledger, err := a.store.Load()
		if err != nil {
			return err
		}
		observed := map[string]state.Observed{}
		if runtime.GOOS == "windows" {
			observed, err = a.engine.Observe(cmd.Context())
			if err != nil {
				return err
			}
		}
		payload := struct {
			SchemaVersion int                       `json:"schema_version"`
			EngineVersion string                    `json:"engine_version"`
			SourceCommit  string                    `json:"source_commit,omitempty"`
			Selections    map[string]catalog.Choice `json:"selections"`
			Components    map[string]state.Observed `json:"components"`
		}{1, ledger.EngineVersion, ledger.SourceCommit, ledger.Selections, observed}
		return printValue(a.out, payload, opts.json)
	}}
	statusCmd.Flags().BoolVar(&opts.json, "json", false, "write stable JSON output")
	root.AddCommand(statusCmd)
	removeCmd := &cobra.Command{Use: "remove [component...]", Args: cobra.MinimumNArgs(1), Short: "Remove owned packages and managed configs", RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "windows" {
			return platform.ErrWindowsOnly
		}
		a, err := run(cmd)
		if err != nil {
			return err
		}
		if !opts.yes && !confirm(cmd.InOrStdin(), a.out, "Remove selected owned items?") {
			return fmt.Errorf("cancelled")
		}
		return a.engine.Remove(cmd.Context(), args, opts.allowDrift)
	}}
	removeCmd.Flags().BoolVar(&opts.yes, "yes", false, "remove without confirmation")
	removeCmd.Flags().BoolVar(&opts.allowDrift, "allow-drift", false, "remove config even when it drifted")
	root.AddCommand(removeCmd)
	root.AddCommand(&cobra.Command{Use: "repair", Short: "Diagnose an interrupted transaction", RunE: func(cmd *cobra.Command, args []string) error {
		a, err := run(cmd)
		if err != nil {
			return err
		}
		return a.engine.Repair(cmd.Context())
	}})
	root.AddCommand(&cobra.Command{Use: "doctor", Short: "Check platform, WinGet, Chezmoi, paths, state, and catalog", RunE: func(cmd *cobra.Command, args []string) error {
		a, err := run(cmd)
		if err != nil {
			return err
		}
		return doctor(cmd.Context(), a)
	}})
	catalogCmd := &cobra.Command{Use: "catalog", Short: "Catalog maintenance"}
	catalogCmd.AddCommand(&cobra.Command{Use: "validate", Short: "Validate manifests, profiles, assets, and compatibility", RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := catalog.Load(content.FS, version); err != nil {
			return err
		}
		if err := catalog.ValidateIntegrity(content.FS); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "catalog valid")
		return nil
	}})
	root.AddCommand(catalogCmd)
	root.AddCommand(&cobra.Command{Use: "capture", Short: "Capture allowlisted Linux setup changes and launch Codex", RunE: func(cmd *cobra.Command, args []string) error { return capture.Run(cmd.InOrStdin(), cmd.OutOrStdout()) }})
	root.AddCommand(&cobra.Command{Use: "update", Short: "Upgrade MySetup through WinGet", RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "windows" {
			return platform.ErrWindowsOnly
		}
		out, err := platform.ExecRunner{}.Run(cmd.Context(), "winget", "upgrade", "--id", "reggieaalbios.MySetup", "-e", "--source", "winget", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity")
		if err != nil {
			return fmt.Errorf("winget update: %w: %s", err, out)
		}
		fmt.Fprint(cmd.OutOrStdout(), string(out))
		return nil
	}})
	syncCmd := &cobra.Command{Use: "sync", Short: "Stage protected main, validate, preview, then apply", RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "windows" {
			return platform.ErrWindowsOnly
		}
		a, err := run(cmd)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(a.store.Root, 0700); err != nil {
			return err
		}
		provider := source.GitHub{Owner: "reggieaalbios", Repo: "my-setup"}
		commit := opts.source
		if commit == "" {
			commit, err = provider.ResolveMain(cmd.Context())
			if err != nil {
				return err
			}
		}
		checkout, _, err := provider.Stage(cmd.Context(), commit, a.store.Root, a.version)
		if err != nil {
			return err
		}
		if err := source.Activate(checkout, a.store.Root, commit); err != nil {
			return err
		}
		ledger, err := a.store.Load()
		if err != nil {
			return err
		}
		ledger.SourceCommit = commit
		if err := a.store.Save(ledger, "state.json"); err != nil {
			return err
		}
		fresh, err := run(cmd)
		if err != nil {
			return err
		}
		selections, profile, err := resolveSelections(fresh.catalog, fresh.store, opts)
		if err != nil {
			return err
		}
		observed, err := fresh.engine.Observe(cmd.Context())
		if err != nil {
			return err
		}
		plan, err := planner.Build(fresh.catalog, selections, observed, profile)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.out, "Activated source commit %s\n", commit)
		if err := printValue(a.out, plan, opts.json); err != nil {
			return err
		}
		if !opts.yes && !confirm(cmd.InOrStdin(), a.out, "Apply synchronized content?") {
			return nil
		}
		if err := platform.EnsureChezmoi(cmd.Context(), filepath.Join(a.store.Root, "tools", "chezmoi.exe")); err != nil {
			return err
		}
		if err := fresh.engine.Apply(cmd.Context(), plan, selections); err != nil {
			return err
		}
		return nil
	}}
	addSelectionFlags(syncCmd)
	root.AddCommand(syncCmd)
	return root
}

func resolveSelections(c catalog.Catalog, store state.Store, o options) (map[string]catalog.Choice, string, error) {
	result := map[string]catalog.Choice{}
	profileName := "Custom"
	if o.profile != "" {
		profile, ok := c.Profiles[strings.ToLower(o.profile)]
		if !ok {
			return nil, "", fmt.Errorf("unknown profile %q", o.profile)
		}
		profileName = profile.Name
		for k, v := range profile.Selections {
			result[k] = v
		}
	} else {
		ledger, err := store.Load()
		if err != nil {
			return nil, "", err
		}
		for k, v := range ledger.Selections {
			result[k] = v
		}
	}
	for _, id := range split(o.packages) {
		choice := result[id]
		choice.Package = true
		result[id] = choice
	}
	for _, id := range split(o.configs) {
		choice := result[id]
		choice.Config = true
		result[id] = choice
	}
	for id := range result {
		if _, ok := c.Components[id]; !ok {
			return nil, "", fmt.Errorf("unknown component %q", id)
		}
	}
	return result, profileName, nil
}
func split(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}
func printValue(w io.Writer, value any, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	switch v := value.(type) {
	case planner.Plan:
		fmt.Fprintf(w, "Plan (%s):\n", v.Profile)
		for _, a := range v.Actions {
			fmt.Fprintf(w, "  %-16s %s\n", a.Kind, a.Name)
		}
		for _, warning := range v.Warnings {
			fmt.Fprintf(w, "  warning: %s\n", warning)
		}
		if len(v.Actions) == 0 {
			fmt.Fprintln(w, "  no changes")
		}
		return nil
	default:
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(data))
		return nil
	}
}
func doctor(ctx context.Context, a *runtimeApp) error {
	failures := 0
	fmt.Fprintf(a.out, "platform: %s\n", runtime.GOOS)
	if runtime.GOOS != "windows" {
		fmt.Fprintln(a.out, "provisioning: unavailable (maintenance-only Linux build)")
	} else {
		if _, err := (platform.ExecRunner{}).Run(ctx, "winget", "--version"); err != nil {
			fmt.Fprintln(a.out, "winget: missing")
			failures++
		} else {
			fmt.Fprintln(a.out, "winget: ok")
		}
	}
	chezmoi := filepath.Join(a.store.Root, "tools", "chezmoi.exe")
	if _, err := os.Stat(chezmoi); err != nil {
		fmt.Fprintf(a.out, "chezmoi: missing private binary at %s\n", chezmoi)
		if runtime.GOOS == "windows" {
			failures++
		}
	} else {
		fmt.Fprintln(a.out, "chezmoi: ok")
	}
	if err := catalog.ValidateIntegrity(content.FS); err != nil {
		fmt.Fprintf(a.out, "catalog: %v\n", err)
		failures++
	} else {
		fmt.Fprintln(a.out, "catalog: ok")
	}
	if failures > 0 {
		return fmt.Errorf("doctor found %d problem(s)", failures)
	}
	return nil
}
func confirm(in io.Reader, out io.Writer, prompt string) bool {
	fmt.Fprintf(out, "%s [y/N] ", prompt)
	line, _ := bufio.NewReader(in).ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(line), "y") || strings.EqualFold(strings.TrimSpace(line), "yes")
}
func terminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}
