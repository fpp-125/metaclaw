package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompileDeterministicManifest(t *testing.T) {
	claw := filepath.Join("..", "..", "testdata", "hello.claw")
	out1 := t.TempDir()
	out2 := t.TempDir()

	res1, err := Compile(claw, out1)
	if err != nil {
		t.Fatalf("Compile #1 failed: %v", err)
	}
	res2, err := Compile(claw, out2)
	if err != nil {
		t.Fatalf("Compile #2 failed: %v", err)
	}
	m1, err := os.ReadFile(filepath.Join(res1.Capsule.Path, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest #1: %v", err)
	}
	m2, err := os.ReadFile(filepath.Join(res2.Capsule.Path, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest #2: %v", err)
	}
	if string(m1) != string(m2) {
		t.Fatalf("expected deterministic manifest output\n#1: %s\n#2: %s", string(m1), string(m2))
	}
}

func TestCompileDeterministicWithOutputUnderSourceRoot(t *testing.T) {
	root := t.TempDir()
	claw := filepath.Join(root, "agent.claw")
	content := `apiVersion: metaclaw/v1
kind: Agent
agent:
  name: hello
  species: nano
  habitat:
    network:
      mode: none
  command:
    - sh
    - -lc
    - echo "hello"
`
	if err := os.WriteFile(claw, []byte(content), 0o644); err != nil {
		t.Fatalf("write clawfile: %v", err)
	}

	out := filepath.Join(root, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}

	res1, err := Compile(claw, out)
	if err != nil {
		t.Fatalf("Compile #1 failed: %v", err)
	}
	m1, err := os.ReadFile(filepath.Join(res1.Capsule.Path, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest #1: %v", err)
	}

	res2, err := Compile(claw, out)
	if err != nil {
		t.Fatalf("Compile #2 failed: %v", err)
	}
	m2, err := os.ReadFile(filepath.Join(res2.Capsule.Path, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest #2: %v", err)
	}

	if string(m1) != string(m2) {
		t.Fatalf("expected deterministic manifest even with in-tree output")
	}
}
