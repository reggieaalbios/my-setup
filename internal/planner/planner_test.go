package planner

import (
	"strings"
	"testing"

	"github.com/reggieaalbios/my-setup/internal/catalog"
	"github.com/reggieaalbios/my-setup/internal/state"
)

func TestConfigOnlyWarns(t *testing.T) {
	c := catalog.Catalog{Components: map[string]catalog.Component{"micro": {ID: "micro", Name: "Micro", Config: &catalog.Config{Targets: []string{"x"}}}}}
	plan, err := Build(c, map[string]catalog.Choice{"micro": {Config: true}}, map[string]state.Observed{}, "Custom")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != ApplyConfig {
		t.Fatalf("actions=%v", plan.Actions)
	}
	if len(plan.Warnings) != 1 || !strings.Contains(plan.Warnings[0], "not installed") {
		t.Fatalf("warnings=%v", plan.Warnings)
	}
}
func TestPackageConfigIndependent(t *testing.T) {
	c := catalog.Catalog{Components: map[string]catalog.Component{"x": {ID: "x", Name: "X", Package: &catalog.Package{ID: "X.X"}, Config: &catalog.Config{Targets: []string{"x"}}}}}
	plan, err := Build(c, map[string]catalog.Choice{"x": {Package: true}}, map[string]state.Observed{}, "Custom")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != InstallPackage {
		t.Fatalf("actions=%v", plan.Actions)
	}
}
