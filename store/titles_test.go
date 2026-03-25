package store

import (
	"testing"
)

func TestEarnTitle(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test_titles.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Earn first title - should be newly earned and set as active
	newlyEarned, err := db.EarnTitle("fp1", "novice")
	if err != nil {
		t.Fatal(err)
	}
	if !newlyEarned {
		t.Error("should be newly earned")
	}

	data, err := db.GetTitleData("fp1")
	if err != nil {
		t.Fatal(err)
	}
	if data.ActiveTitle != "novice" {
		t.Errorf("active title should be novice, got %s", data.ActiveTitle)
	}
	if len(data.Earned) != 1 {
		t.Errorf("should have 1 earned title, got %d", len(data.Earned))
	}

	// Earn second title - active should remain first
	newlyEarned2, err := db.EarnTitle("fp1", "explorer")
	if err != nil {
		t.Fatal(err)
	}
	if !newlyEarned2 {
		t.Error("explorer should be newly earned")
	}

	data2, err := db.GetTitleData("fp1")
	if err != nil {
		t.Fatal(err)
	}
	if data2.ActiveTitle != "novice" {
		t.Errorf("active title should remain novice, got %s", data2.ActiveTitle)
	}
	if len(data2.Earned) != 2 {
		t.Errorf("should have 2 earned titles, got %d", len(data2.Earned))
	}

	// Duplicate earn returns false
	dup, err := db.EarnTitle("fp1", "novice")
	if err != nil {
		t.Fatal(err)
	}
	if dup {
		t.Error("duplicate earn should return false")
	}
}

func TestSetActiveTitle(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test_titles_active.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.EarnTitle("fp1", "novice")
	db.EarnTitle("fp1", "explorer")

	err = db.SetActiveTitle("fp1", "explorer")
	if err != nil {
		t.Fatal(err)
	}

	data, err := db.GetTitleData("fp1")
	if err != nil {
		t.Fatal(err)
	}
	if data.ActiveTitle != "explorer" {
		t.Errorf("active title should be explorer, got %s", data.ActiveTitle)
	}

	// Setting unearned title should be a no-op
	db.SetActiveTitle("fp1", "champion")
	data2, _ := db.GetTitleData("fp1")
	if data2.ActiveTitle != "explorer" {
		t.Errorf("active title should remain explorer, got %s", data2.ActiveTitle)
	}
}

func TestGetTitleDataEmpty(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test_titles_empty.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	data, err := db.GetTitleData("fp_unknown")
	if err != nil {
		t.Fatal(err)
	}
	if data.ActiveTitle != "" {
		t.Errorf("expected empty active title, got %s", data.ActiveTitle)
	}
	if len(data.Earned) != 0 {
		t.Errorf("expected no earned titles, got %d", len(data.Earned))
	}
}
