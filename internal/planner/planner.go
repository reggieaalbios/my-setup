package planner

import (
	"fmt"
	"sort"

	"github.com/reggieaalbios/my-setup/internal/catalog"
	"github.com/reggieaalbios/my-setup/internal/state"
)

type ActionKind string

const (
	InstallPackage ActionKind = "install_package"
	ApplyConfig    ActionKind = "apply_config"
	RemovePackage  ActionKind = "remove_package"
	RemoveConfig   ActionKind = "remove_config"
)

type Action struct {
	Component string     `json:"component"`
	Name      string     `json:"name"`
	Kind      ActionKind `json:"kind"`
	PackageID string     `json:"package_id,omitempty"`
	Targets   []string   `json:"targets,omitempty"`
}
type Plan struct {
	SchemaVersion int      `json:"schema_version"`
	Profile       string   `json:"profile"`
	Actions       []Action `json:"actions"`
	Warnings      []string `json:"warnings"`
}

func Build(c catalog.Catalog, selections map[string]catalog.Choice, observed map[string]state.Observed, profile string) (Plan, error) {
	resolved := clone(selections)
	queue := []string{}
	for id, choice := range resolved {
		if choice.Package {
			queue = append(queue, id)
		}
	}
	seen := map[string]bool{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		component, ok := c.Components[id]
		if !ok {
			return Plan{}, fmt.Errorf("unknown component %q", id)
		}
		for _, dependency := range component.Required {
			dep := resolved[dependency]
			if !dep.Package {
				dep.Package = true
				resolved[dependency] = dep
			}
			queue = append(queue, dependency)
		}
	}
	plan := Plan{SchemaVersion: 1, Profile: profile, Actions: []Action{}, Warnings: []string{}}
	ids := make([]string, 0, len(resolved))
	for id := range resolved {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		choice := resolved[id]
		component, ok := c.Components[id]
		if !ok {
			return Plan{}, fmt.Errorf("unknown component %q", id)
		}
		obs := observed[id]
		if choice.Package && component.Package != nil && !obs.Installed {
			plan.Actions = append(plan.Actions, Action{Component: id, Name: component.Name, Kind: InstallPackage, PackageID: component.Package.ID})
		}
		if choice.Config && component.Config != nil {
			plan.Actions = append(plan.Actions, Action{Component: id, Name: component.Name, Kind: ApplyConfig, Targets: append([]string(nil), component.Config.Targets...)})
			if !choice.Package && !obs.Installed {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s config selected while its application is not installed", component.Name))
			}
		}
		for _, dep := range component.Recommended {
			if !resolved[dep].Package {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s recommends %s", component.Name, c.Components[dep].Name))
			}
		}
	}
	return plan, nil
}

func clone(in map[string]catalog.Choice) map[string]catalog.Choice {
	out := make(map[string]catalog.Choice, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
