package capture

import "testing"

func TestAllowlistAndRejection(t *testing.T) {
	if !allow("content/components/new.json") {
		t.Fatal("managed content rejected")
	}
	if allow("README.md") {
		t.Fatal("broad unlisted path allowed")
	}
	for _, name := range []string{"content/token.json", "content/History", "content/cache/db"} {
		if !rejected(name) {
			t.Fatalf("sensitive path %q accepted", name)
		}
	}
}
