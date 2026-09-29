package catalog_test

import (
	"testing"

	"github.com/reggieaalbios/my-setup/content"
	"github.com/reggieaalbios/my-setup/internal/catalog"
)

func TestEmbeddedCatalog(t *testing.T) {
	c, err := catalog.Load(content.FS, "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(c.Components); got != 16 {
		t.Fatalf("components=%d want 16", got)
	}
	if got := len(c.Profiles["custom"].Selections); got != 0 {
		t.Fatalf("custom first run has %d selections", got)
	}
	if !c.Profiles["minimal"].Selections["micro"].Config {
		t.Fatal("minimal should select Micro config")
	}
}
func TestIntegrity(t *testing.T) {
	if err := catalog.ValidateIntegrity(content.FS); err != nil {
		t.Fatal(err)
	}
}
