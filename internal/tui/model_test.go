package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/reggieaalbios/my-setup/internal/catalog"
)

func TestStartsEmptyAndTogglesAdjacentChoices(t *testing.T) {
	c := catalog.Catalog{Components: map[string]catalog.Component{"x": {ID: "x", Name: "X", Category: "CLI", Package: &catalog.Package{ID: "X.X"}, Config: &catalog.Config{Targets: []string{"x"}}}}, Profiles: map[string]catalog.Profile{}}
	m := New(c)
	if len(m.Choices) != 0 {
		t.Fatal("first run must be empty")
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: ' '})
	m = updated.(Model)
	if !m.Choices["x"].Package || m.Choices["x"].Config {
		t.Fatalf("choice=%+v", m.Choices["x"])
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: ' '})
	m = updated.(Model)
	if !m.Choices["x"].Config {
		t.Fatal("config did not toggle")
	}
}
