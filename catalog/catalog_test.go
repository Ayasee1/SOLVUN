package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReadsJSONFilesSorted(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "b.json", `{"id":2,"name":"B","items":[{"id":2,"name":"Item","price":10,"stock":1}]}`)
	writeFile(t, dir, "a.json", `{"id":1,"name":"A","items":[{"id":1,"name":"First","price":5,"stock":2}]}`)
	writeFile(t, dir, "notes.txt", "this file must be ignored")

	catalogs, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalogs) != 2 {
		t.Fatalf("loaded %d catalogs, want 2", len(catalogs))
	}
	if catalogs[0].ID != 1 || catalogs[1].ID != 2 {
		t.Errorf("catalogs are not sorted by id: %+v", catalogs)
	}
	if catalogs[0].Items[0].Name != "First" {
		t.Errorf("first item = %q, want First", catalogs[0].Items[0].Name)
	}
}

func TestLoadInvalidJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.json", `{oops`)

	if _, err := Load(dir); err == nil {
		t.Error("expected an error for invalid json, got nil")
	}
}

func TestLoadMissingDirReturnsError(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected an error for a missing directory, got nil")
	}
}

func TestShippedCatalogFilesAreValid(t *testing.T) {
	catalogs, err := Load(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalogs) == 0 {
		t.Fatal("no catalog files found")
	}

	for _, catalog := range catalogs {
		for _, item := range catalog.Items {
			if item.ID == 0 || item.Name == "" {
				t.Errorf("catalog %q has an item without id/name: %+v", catalog.Name, item)
			}
		}
	}
}
