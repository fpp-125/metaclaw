package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func writeManifest(t *testing.T, templateDir string, managed, user []string) {
	t.Helper()
	m := TemplateManifest{
		SchemaVersion: 1,
		ID:            "test-template",
		Managed:       managed,
		User:          user,
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	writeFile(t, filepath.Join(templateDir, ManifestFilename), string(b)+"\n")
}

func TestUpgrade_SkipsWhenAlreadyUpToDate(t *testing.T) {
	tmp := t.TempDir()
	templateDir := filepath.Join(tmp, "template")
	projectDir := filepath.Join(tmp, "project")

	writeManifest(t, templateDir, []string{"bot/**", "README.md"}, nil)
	writeFile(t, filepath.Join(templateDir, "README.md"), "v1\n")
	writeFile(t, filepath.Join(templateDir, "bot", "chat_once.py"), "print('v1')\n")

	res1, err := Upgrade(UpgradeOptions{
		ProjectDir: projectDir,
		Template: TemplateSource{
			Kind: TemplateSourceKindLocal,
			Dir:  templateDir,
		},
	})
	if err != nil {
		t.Fatalf("upgrade v1: %v", err)
	}
	if len(res1.Added) != 2 || len(res1.Updated) != 0 || len(res1.Conflicts) != 0 {
		t.Fatalf("unexpected result v1: %+v", res1)
	}

	// Second upgrade with the same template should skip (not "update") because dst already matches src.
	res2, err := Upgrade(UpgradeOptions{
		ProjectDir: projectDir,
		Template: TemplateSource{
			Kind: TemplateSourceKindLocal,
			Dir:  templateDir,
		},
	})
	if err != nil {
		t.Fatalf("upgrade v1 again: %v", err)
	}
	if len(res2.Skipped) != 2 || len(res2.Updated) != 0 || len(res2.Added) != 0 || len(res2.Conflicts) != 0 {
		t.Fatalf("expected skip-only result, got: %+v", res2)
	}

	// Update template: should produce "updated".
	writeFile(t, filepath.Join(templateDir, "README.md"), "v2\n")
	res3, err := Upgrade(UpgradeOptions{
		ProjectDir: projectDir,
		Template: TemplateSource{
			Kind: TemplateSourceKindLocal,
			Dir:  templateDir,
		},
	})
	if err != nil {
		t.Fatalf("upgrade v2: %v", err)
	}
	if len(res3.Updated) != 1 || len(res3.Added) != 0 || len(res3.Conflicts) != 0 {
		t.Fatalf("expected 1 updated file, got: %+v", res3)
	}

	// Local modification should conflict (when it diverges from both lock and template).
	writeFile(t, filepath.Join(projectDir, "README.md"), "local-change\n")
	_, err = Upgrade(UpgradeOptions{
		ProjectDir: projectDir,
		Template: TemplateSource{
			Kind: TemplateSourceKindLocal,
			Dir:  templateDir,
		},
	})
	if err == nil {
		t.Fatalf("expected conflict error, got nil")
	}
}

func TestUpgrade_NoConflictWhenUserAlreadyAppliedTemplate(t *testing.T) {
	tmp := t.TempDir()
	templateDir := filepath.Join(tmp, "template")
	projectDir := filepath.Join(tmp, "project")

	writeManifest(t, templateDir, []string{"README.md"}, nil)
	writeFile(t, filepath.Join(templateDir, "README.md"), "v1\n")

	if _, err := Upgrade(UpgradeOptions{
		ProjectDir: projectDir,
		Template: TemplateSource{
			Kind: TemplateSourceKindLocal,
			Dir:  templateDir,
		},
	}); err != nil {
		t.Fatalf("upgrade v1: %v", err)
	}

	// Change template to v2, but also change dst to v2 *before* upgrading.
	writeFile(t, filepath.Join(templateDir, "README.md"), "v2\n")
	writeFile(t, filepath.Join(projectDir, "README.md"), "v2\n")

	res, err := Upgrade(UpgradeOptions{
		ProjectDir: projectDir,
		Template: TemplateSource{
			Kind: TemplateSourceKindLocal,
			Dir:  templateDir,
		},
	})
	if err != nil {
		t.Fatalf("upgrade should not conflict: %v", err)
	}
	if len(res.Conflicts) != 0 {
		t.Fatalf("expected no conflicts, got: %+v", res)
	}
	if len(res.Skipped) != 1 {
		t.Fatalf("expected skip (already matches template), got: %+v", res)
	}
}

