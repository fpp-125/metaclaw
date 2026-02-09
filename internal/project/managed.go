package project

import (
	"fmt"
	"path/filepath"
	"sort"
)

// ManagedFiles expands a template manifest into a stable, slash-separated list of managed file paths.
func ManagedFiles(templateDir string, manifest TemplateManifest) ([]string, error) {
	files, err := expandManagedFiles(templateDir, manifest.Managed, manifest.User)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// HashManagedFiles computes sha256 hashes for the given slash-separated relative paths.
func HashManagedFiles(projectDir string, managed []string) (map[string]string, error) {
	out := map[string]string{}
	for _, rel := range managed {
		dst := filepath.Join(projectDir, filepath.FromSlash(rel))
		sum, err := sha256File(dst)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", rel, err)
		}
		out[rel] = sum
	}
	return out, nil
}
