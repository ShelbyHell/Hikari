package ready

import (
	"strings"
	"testing"
)

func TestReadyTemplates(t *testing.T) {
	all := All()
	if len(all) < 3 {
		t.Fatalf("expected at least 3 templates, got %d", len(all))
	}
	for _, item := range all {
		got, ok := Get(item.ID)
		if !ok {
			t.Fatalf("Get(%q) not found", item.ID)
		}
		if got.ID != item.ID {
			t.Fatalf("ID mismatch: %s != %s", got.ID, item.ID)
		}
		if !item.Starter && strings.TrimSpace(item.Body) == "" {
			t.Fatalf("template %q has empty body", item.ID)
		}
	}
}
