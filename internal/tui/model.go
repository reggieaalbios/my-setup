package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/reggieaalbios/my-setup/internal/catalog"
)

type Model struct {
	Catalog        catalog.Catalog
	Items          []catalog.Component
	Choices        map[string]catalog.Choice
	Cursor, Column int
	Profile        string
	Section        int
	Submit         bool
	Remove         map[string]bool
}

func New(c catalog.Catalog) Model {
	return Model{Catalog: c, Items: catalog.SortedComponents(c), Choices: map[string]catalog.Choice{}, Profile: "Custom", Remove: map[string]bool{}}
}
func (m Model) Init() tea.Cmd { return nil }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(m.Items)-1 {
			m.Cursor++
		}
	case "left", "h":
		m.Column = 0
	case "right", "l":
		m.Column = 1
	case " ", "space", "enter":
		if len(m.Items) > 0 {
			item := m.Items[m.Cursor]
			choice := m.Choices[item.ID]
			if m.Column == 0 && item.Package != nil {
				choice.Package = !choice.Package
			}
			if m.Column == 1 && item.Config != nil {
				choice.Config = !choice.Config
			}
			m.Choices[item.ID] = choice
		}
	case "1":
		m = m.WithProfile("minimal")
	case "2":
		m = m.WithProfile("developer")
	case "3":
		m = m.WithProfile("full")
	case "0":
		m = m.WithProfile("custom")
	case "tab":
		m.Section = (m.Section + 1) % 4
	case "shift+tab":
		m.Section = (m.Section + 3) % 4
	case "x":
		if m.Section == 3 && len(m.Items) > 0 {
			m.Remove[m.Items[m.Cursor].ID] = !m.Remove[m.Items[m.Cursor].ID]
		}
	case "a":
		if m.Section == 1 {
			m.Submit = true
			return m, tea.Quit
		}
	}
	return m, nil
}
func (m Model) View() tea.View {
	var b strings.Builder
	sections := []string{"Tools", "Review", "Status", "Remove"}
	fmt.Fprintf(&b, "MySetup — %s profile — %s\n\n", m.Profile, sections[m.Section])
	if m.Section == 1 {
		selected := 0
		for _, choice := range m.Choices {
			if choice.Package || choice.Config {
				selected++
			}
		}
		fmt.Fprintf(&b, "%d components selected. Press a to apply or tab to continue.\n", selected)
		return tea.NewView(b.String())
	}
	b.WriteString("Tool                     Package     Config\n")
	for i, item := range m.Items {
		cursor := " "
		if i == m.Cursor {
			cursor = ">"
		}
		if m.Section == 3 && m.Remove[item.ID] {
			cursor = "x"
		}
		choice := m.Choices[item.ID]
		pkg, cfg := "—", "—"
		if item.Package != nil {
			pkg = box(choice.Package)
		}
		if item.Config != nil {
			cfg = box(choice.Config)
		}
		fmt.Fprintf(&b, "%s %-24s %-11s %s\n", cursor, item.Name, pkg, cfg)
	}
	b.WriteString("\n↑/↓ select  ←/→ column  space toggle  tab section  x remove  0-3 profile  q quit\n")
	return tea.NewView(b.String())
}
func (m Model) WithProfile(name string) Model {
	profile, ok := m.Catalog.Profiles[strings.ToLower(name)]
	if !ok {
		return m
	}
	m.Profile = profile.Name
	m.Choices = map[string]catalog.Choice{}
	for k, v := range profile.Selections {
		m.Choices[k] = v
	}
	return m
}
func box(v bool) string {
	if v {
		return "[x]"
	}
	return "[ ]"
}
