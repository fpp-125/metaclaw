package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceFirstNetworkMode(t *testing.T) {
	in := "agent:\n  habitat:\n    network:\n      mode: none\n    mounts: []\n"
	out := replaceFirstNetworkMode(in, "outbound")
	if !strings.Contains(out, "mode: outbound") {
		t.Fatalf("expected outbound mode, got: %s", out)
	}
	if strings.Count(out, "mode:") != 1 {
		t.Fatalf("expected single mode entry, got: %s", out)
	}
}

func TestRewriteObsidianAgentFile(t *testing.T) {
	dir := t.TempDir()
	agent := filepath.Join(dir, "agent.claw")
	content := `apiVersion: metaclaw/v1
kind: Agent
agent:
  habitat:
    network:
      mode: none
    mounts:
      - source: /ABS/PATH/TO/OBSIDIAN_VAULT
        target: /vault
      - source: /ABS/PATH/TO/BOT_HOST_DATA/runtime
        target: /runtime
`
	if err := os.WriteFile(agent, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := rewriteObsidianAgentFile(agent, "/vault/path", "/bot/data", "outbound"); err != nil {
		t.Fatalf("rewrite agent: %v", err)
	}
	b, err := os.ReadFile(agent)
	if err != nil {
		t.Fatalf("read rewritten agent: %v", err)
	}
	text := string(b)
	if !strings.Contains(text, "source: /vault/path") {
		t.Fatalf("vault path not replaced: %s", text)
	}
	if !strings.Contains(text, "source: /bot/data/runtime") {
		t.Fatalf("host data path not replaced: %s", text)
	}
	if !strings.Contains(text, "mode: outbound") {
		t.Fatalf("network mode not replaced: %s", text)
	}
}

func TestRewriteQuickstartChatScript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat.sh")
	input := `#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
export BOT_RENDER_MODE="${BOT_RENDER_MODE:-glow}"
export BOT_NETWORK_MODE="${BOT_NETWORK_MODE:-none}"
exec python3 "$PROJECT_DIR/chat_tui.py" "$@"
`
	if err := os.WriteFile(path, []byte(input), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	profile, ok := resolveObsidianProfile("obsidian-research")
	if !ok {
		t.Fatal("expected obsidian-research profile")
	}
	if err := rewriteQuickstartChatScript(path, "/tmp/metaclaw-project/.metaclaw", "GEMINI_API_KEY", "TAVILY_API_KEY", profile); err != nil {
		t.Fatalf("rewrite chat.sh: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rewritten chat.sh: %v", err)
	}
	text := string(b)
	if !strings.Contains(text, "${BOT_NETWORK_MODE:-outbound}") {
		t.Fatalf("expected outbound network default: %s", text)
	}
	if !strings.Contains(text, "BOT_HOST_DATA_DIR") {
		t.Fatalf("expected host data export: %s", text)
	}
	if !strings.Contains(text, "LLM_KEY_ENV") || !strings.Contains(text, "TAVILY_KEY_ENV") {
		t.Fatalf("expected key env exports: %s", text)
	}
}

func TestWriteObsidianProfileDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui.defaults.json")
	profile, ok := resolveObsidianProfile("obsidian-chat")
	if !ok {
		t.Fatal("expected obsidian-chat profile")
	}
	if err := writeObsidianProfileDefaults(path, profile); err != nil {
		t.Fatalf("write profile defaults: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read defaults file: %v", err)
	}
	payload := map[string]string{}
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatalf("parse defaults json: %v", err)
	}
	if payload["network_mode"] != "none" {
		t.Fatalf("network_mode mismatch: %+v", payload)
	}
	if payload["render_mode"] != "glow" {
		t.Fatalf("render_mode mismatch: %+v", payload)
	}
	if payload["retrieval_scope"] != "limited" {
		t.Fatalf("retrieval_scope mismatch: %+v", payload)
	}
}

func TestResolveRequestedRuntimeRejectsInvalid(t *testing.T) {
	_, _, err := resolveRequestedRuntime("not-a-runtime")
	if err == nil {
		t.Fatal("expected invalid runtime error")
	}
}
