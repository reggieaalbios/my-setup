package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

const SchemaVersion = 1

type Package struct {
	ID     string   `json:"id"`
	Scope  string   `json:"scope,omitempty"`
	Detect []string `json:"detect,omitempty"`
	Verify []string `json:"verify,omitempty"`
}

type Config struct {
	Source       string   `json:"source"`
	Destinations []string `json:"destinations"`
	Targets      []string `json:"targets"`
	Verify       []string `json:"verify,omitempty"`
	Exclusions   []string `json:"exclusions,omitempty"`
}

type Component struct {
	SchemaVersion int      `json:"schema_version"`
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	Platforms     []string `json:"platforms"`
	Package       *Package `json:"package,omitempty"`
	Config        *Config  `json:"config,omitempty"`
	Required      []string `json:"required,omitempty"`
	Recommended   []string `json:"recommended,omitempty"`
	MinimumEngine string   `json:"minimum_engine_version"`
}

type Choice struct {
	Package bool `json:"package"`
	Config  bool `json:"config"`
}

type Profile struct {
	SchemaVersion int               `json:"schema_version"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Selections    map[string]Choice `json:"selections"`
}

type Integrity struct {
	SchemaVersion int               `json:"schema_version"`
	Files         map[string]string `json:"files"`
}

type Catalog struct {
	Components map[string]Component
	Profiles   map[string]Profile
}

func Load(fsys fs.FS, engineVersion string) (Catalog, error) {
	c := Catalog{Components: map[string]Component{}, Profiles: map[string]Profile{}}
	componentFiles, err := fs.Glob(fsys, "components/*.json")
	if err != nil {
		return c, err
	}
	for _, name := range componentFiles {
		var item Component
		if err := decode(fsys, name, &item); err != nil {
			return c, err
		}
		if err := validateComponent(item, engineVersion); err != nil {
			return c, fmt.Errorf("%s: %w", name, err)
		}
		if _, exists := c.Components[item.ID]; exists {
			return c, fmt.Errorf("duplicate component %q", item.ID)
		}
		c.Components[item.ID] = item
	}
	profileFiles, err := fs.Glob(fsys, "profiles/*.json")
	if err != nil {
		return c, err
	}
	for _, name := range profileFiles {
		var profile Profile
		if err := decode(fsys, name, &profile); err != nil {
			return c, err
		}
		if profile.SchemaVersion != SchemaVersion || profile.Name == "" {
			return c, fmt.Errorf("%s: invalid profile", name)
		}
		for id, choice := range profile.Selections {
			component, ok := c.Components[id]
			if !ok {
				return c, fmt.Errorf("%s references unknown component %q", name, id)
			}
			if choice.Package && component.Package == nil {
				return c, fmt.Errorf("%s selects missing package for %q", name, id)
			}
			if choice.Config && component.Config == nil {
				return c, fmt.Errorf("%s selects missing config for %q", name, id)
			}
		}
		c.Profiles[strings.ToLower(profile.Name)] = profile
	}
	if len(c.Components) == 0 {
		return c, errors.New("catalog has no components")
	}
	for id, component := range c.Components {
		for _, dependency := range append(append([]string{}, component.Required...), component.Recommended...) {
			if _, ok := c.Components[dependency]; !ok {
				return c, fmt.Errorf("component %q references unknown dependency %q", id, dependency)
			}
		}
	}
	if err := validateDependencyCycles(c.Components); err != nil {
		return c, err
	}
	return c, nil
}

func ValidateIntegrity(fsys fs.FS) error {
	var index Integrity
	if err := decode(fsys, "integrity.json", &index); err != nil {
		return err
	}
	if index.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported integrity schema %d", index.SchemaVersion)
	}
	for name, expected := range index.Files {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("integrity %s: %w", name, err)
		}
		sum := sha256.Sum256(data)
		if actual := hex.EncodeToString(sum[:]); actual != expected {
			return fmt.Errorf("integrity mismatch for %s", name)
		}
	}
	seen := map[string]bool{}
	for name := range index.Files {
		seen[name] = true
	}
	for _, root := range []string{"components", "profiles", "chezmoi"} {
		if err := fs.WalkDir(fsys, root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if !seen[name] {
				return fmt.Errorf("integrity index missing %s", name)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func SortedComponents(c Catalog) []Component {
	out := make([]Component, 0, len(c.Components))
	for _, component := range c.Components {
		out = append(out, component)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category == out[j].Category {
			return out[i].Name < out[j].Name
		}
		return out[i].Category < out[j].Category
	})
	return out
}

func decode(fsys fs.FS, name string, dst any) error {
	data, err := fs.ReadFile(fsys, path.Clean(name))
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func validateComponent(c Component, engine string) error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema %d", c.SchemaVersion)
	}
	if c.ID == "" || c.Name == "" || c.Category == "" || len(c.Platforms) == 0 {
		return errors.New("id, name, category, and platforms are required")
	}
	if c.Package == nil && c.Config == nil {
		return errors.New("component must provide package or config")
	}
	if c.Package != nil && c.Package.ID == "" {
		return errors.New("package id is required")
	}
	if c.Config != nil && (c.Config.Source == "" || len(c.Config.Targets) == 0) {
		return errors.New("config source and targets are required")
	}
	if c.Config != nil {
		for _, target := range c.Config.Targets {
			clean := path.Clean(strings.ReplaceAll(target, "\\", "/"))
			if target == "" || clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || strings.Contains(clean, ":") {
				return fmt.Errorf("unsafe config target %q", target)
			}
		}
	}
	if c.MinimumEngine == "" {
		return errors.New("minimum_engine_version is required")
	}
	if engine != "dev" && compareSemver(engine, c.MinimumEngine) < 0 {
		return fmt.Errorf("requires engine %s (have %s)", c.MinimumEngine, engine)
	}
	return nil
}

func validateDependencyCycles(components map[string]Component) error {
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("required dependency cycle at %q", id)
		}
		if done[id] {
			return nil
		}
		visiting[id] = true
		for _, dep := range components[id].Required {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[id] = false
		done[id] = true
		return nil
	}
	for id := range components {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func compareSemver(a, b string) int {
	trim := func(s string) []int {
		s = strings.TrimPrefix(s, "v")
		var x, y, z int
		fmt.Sscanf(strings.SplitN(s, "-", 2)[0], "%d.%d.%d", &x, &y, &z)
		return []int{x, y, z}
	}
	aa, bb := trim(a), trim(b)
	for i := range aa {
		if aa[i] < bb[i] {
			return -1
		}
		if aa[i] > bb[i] {
			return 1
		}
	}
	return 0
}
