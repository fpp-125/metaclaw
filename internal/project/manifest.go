package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// TemplateManifest describes which files are managed by MetaClaw upgrades vs user-owned.
//
// The manifest lives inside the template directory as `metaclaw.template.json`.
// It is intentionally simple so templates remain portable across repos.
type TemplateManifest struct {
	SchemaVersion int      `json:"schemaVersion"`
	ID            string   `json:"id"`
	Managed       []string `json:"managed"`
	User          []string `json:"user,omitempty"`
}

const ManifestFilename = "metaclaw.template.json"

func LoadManifest(templateDir string) (TemplateManifest, error) {
	path := filepath.Join(templateDir, ManifestFilename)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TemplateManifest{}, fmt.Errorf("template manifest missing: %s", path)
		}
		return TemplateManifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m TemplateManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return TemplateManifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if m.SchemaVersion == 0 {
		m.SchemaVersion = 1
	}
	if m.SchemaVersion != 1 {
		return TemplateManifest{}, fmt.Errorf("unsupported manifest schemaVersion %d", m.SchemaVersion)
	}
	if m.ID == "" {
		return TemplateManifest{}, fmt.Errorf("manifest id is required (%s)", path)
	}
	if len(m.Managed) == 0 {
		return TemplateManifest{}, fmt.Errorf("manifest managed list is empty (%s)", path)
	}
	return m, nil
}
