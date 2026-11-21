package api

import (
	"testing"

	"mikan/internal/panel/subs/ready"
)

func TestReadyTemplatesCatalog(t *testing.T) {
	templates := ready.All()
	if len(templates) < 3 {
		t.Fatalf("expected at least 3 ready templates, got %d", len(templates))
	}
	tpl, ok := ready.Get("privwl-blocked")
	if !ok {
		t.Fatal("privwl-blocked not found")
	}
	if len(tpl.Body) == 0 {
		t.Fatal("privwl-blocked has empty body")
	}

	tpl2, ok2 := ready.Get("davoyan-ru-direct")
	if !ok2 {
		t.Fatal("davoyan-ru-direct not found")
	}
	if len(tpl2.Body) == 0 {
		t.Fatal("davoyan-ru-direct has empty body")
	}
}

func TestReadyView(t *testing.T) {
	tpl, _ := ready.Get("privwl-blocked")
	v := readyView(tpl)
	if v.ID != "privwl-blocked" || v.Author != "Nemu-x" {
		t.Fatalf("unexpected view: %+v", v)
	}
}
