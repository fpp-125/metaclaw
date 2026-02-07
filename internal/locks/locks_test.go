package locks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashSkillPathIncludesContractForFileSkill(t *testing.T) {
	root := t.TempDir()
	skillFile := filepath.Join(root, "skill.sh")
	if err := os.WriteFile(skillFile, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	contractPath := filepath.Join(root, "capability.contract.yaml")
	contractV1 := `apiVersion: metaclaw.capability/v1
kind: CapabilityContract
metadata:
  name: obsidian.demo
  version: v1.0.0
permissions:
  network: none
`
	if err := os.WriteFile(contractPath, []byte(contractV1), 0o644); err != nil {
		t.Fatalf("write contract: %v", err)
	}

	h1, err := hashSkillPath(skillFile)
	if err != nil {
		t.Fatalf("hashSkillPath() error = %v", err)
	}

	contractV2 := `apiVersion: metaclaw.capability/v1
kind: CapabilityContract
metadata:
  name: obsidian.demo
  version: v2.0.0
permissions:
  network: none
`
	if err := os.WriteFile(contractPath, []byte(contractV2), 0o644); err != nil {
		t.Fatalf("rewrite contract: %v", err)
	}
	h2, err := hashSkillPath(skillFile)
	if err != nil {
		t.Fatalf("hashSkillPath() second error = %v", err)
	}
	if h1 == h2 {
		t.Fatal("expected skill digest to change when capability contract changes")
	}
}

func TestBuildSourceLockRejectsSymlinkOutsideSourceRoot(t *testing.T) {
	root := t.TempDir()
	external := filepath.Join(t.TempDir(), "external.txt")
	if err := os.WriteFile(external, []byte("outside"), 0o644); err != nil {
		t.Fatalf("write external file: %v", err)
	}
	link := filepath.Join(root, "external_link")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symlink is not supported in this environment: %v", err)
	}

	_, err := buildSourceLock(root, nil)
	if err == nil {
		t.Fatal("expected source lock generation to reject symlink outside source root")
	}
	if !strings.Contains(err.Error(), "outside source root") {
		t.Fatalf("expected outside source root error, got: %v", err)
	}
}

func TestBuildSourceLockAllowsInternalSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "data.txt")
	if err := os.WriteFile(target, []byte("inside"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	link := filepath.Join(root, "data_link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink is not supported in this environment: %v", err)
	}

	lock, err := buildSourceLock(root, nil)
	if err != nil {
		t.Fatalf("buildSourceLock() error = %v", err)
	}
	found := false
	for _, f := range lock.Files {
		if f.Path == "data_link" {
			found = true
			if f.SHA256 == "" {
				t.Fatal("expected symlink file hash to be non-empty")
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected source lock manifest to include symlink entry")
	}
}
