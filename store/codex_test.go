package store

import (
	"testing"
)

func TestRecordCodexEntry(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test_codex.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// Record some entries
	_ = db.RecordCodexEntry("fp1", "monster", "goblin")
	_ = db.RecordCodexEntry("fp1", "monster", "skeleton")
	_ = db.RecordCodexEntry("fp1", "item", "health_potion")
	_ = db.RecordCodexEntry("fp1", "event", "altar")

	codex, err := db.GetCodex("fp1")
	if err != nil {
		t.Fatal(err)
	}

	if len(codex.Monsters) != 2 {
		t.Errorf("expected 2 monsters, got %d", len(codex.Monsters))
	}
	if !codex.Monsters["goblin"] {
		t.Error("expected goblin in monsters")
	}
	if !codex.Monsters["skeleton"] {
		t.Error("expected skeleton in monsters")
	}
	if len(codex.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(codex.Items))
	}
	if !codex.Items["health_potion"] {
		t.Error("expected health_potion in items")
	}
	if len(codex.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(codex.Events))
	}
	if !codex.Events["altar"] {
		t.Error("expected altar in events")
	}

	// Duplicate entry should not increase count
	_ = db.RecordCodexEntry("fp1", "monster", "goblin")
	codex2, _ := db.GetCodex("fp1")
	if len(codex2.Monsters) != 2 {
		t.Errorf("expected still 2 monsters after duplicate, got %d", len(codex2.Monsters))
	}
}

func TestGetCodexEmpty(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test_codex_empty.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	codex, err := db.GetCodex("fp_unknown")
	if err != nil {
		t.Fatal(err)
	}
	if len(codex.Monsters) != 0 || len(codex.Items) != 0 || len(codex.Events) != 0 {
		t.Error("expected empty codex for unknown player")
	}
}
