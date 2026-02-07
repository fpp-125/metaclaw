package locks

import (
	"os"
	"path/filepath"
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
