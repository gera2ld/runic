package db

import (
	"os"
	"testing"
)

func TestGetHistoryByIDs(t *testing.T) {
	tmpFile := "test_history.db"
	defer os.Remove(tmpFile)

	db, err := Open(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id1, _ := db.InsertHistory("action1", "log1")
	db.InsertHistory("action2", "log2")
	id3, _ := db.InsertHistory("action3", "log3")

	entries, err := db.GetHistoryByIDs([]int64{id1, id3})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Should be ordered by ID DESC
	if entries[0].ID != id3 || entries[1].ID != id1 {
		t.Errorf("expected IDs %d, %d; got %d, %d", id3, id1, entries[0].ID, entries[1].ID)
	}
}
