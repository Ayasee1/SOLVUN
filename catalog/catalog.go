// Package catalog loads product content from JSON files, so product details
// can be edited without touching Go code.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golearn/model"
)

// Load reads every .json file in dir. Each file describes one catalog.
func Load(dir string) ([]model.Catalog, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read catalog dir %q: %w", dir, err)
	}

	var catalogs []model.Catalog
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		var catalog model.Catalog
		if err := json.Unmarshal(data, &catalog); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		catalogs = append(catalogs, catalog)
	}

	sort.Slice(catalogs, func(i, j int) bool { return catalogs[i].ID < catalogs[j].ID })
	return catalogs, nil
}
