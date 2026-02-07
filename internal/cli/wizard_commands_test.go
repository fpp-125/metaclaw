package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWizardGeneratesObsidianScaffold(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "obsidian.claw")
	vault := filepath.Join(root, "vault")
	config := filepath.Join(root, "config")
	logs := filepath.Join(root, "logs")

	code := runWizard([]string{
		"--out", out,
		"--agent-name", "quant-research-bot",
		"--vault", vault,
		"--config-dir", config,
		"--logs-dir", logs,
		"--provider", "gemini_openai",
		"--model", "gemini-2.5-pro",
		"--network", "outbound",
		"--lifecycle", "daemon",
	})
	if code != 0 {
		t.Fatalf("runWizard() code = %d, want 0", code)
	}

	if _, err := os.Stat(vault); err != nil {
		t.Fatalf("expected vault directory to be created: %v", err)
	}
	if _, err := os.Stat(config); err != nil {
		t.Fatalf("expected config directory to be created: %v", err)
	}
	if _, err := os.Stat(logs); err != nil {
		t.Fatalf("expected logs directory to be created: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read generated clawfile: %v", err)
	}
	text := string(b)
	if !strings.Contains(text, "name: quant-research-bot") {
		t.Fatalf("expected agent name in output: %s", text)
	}
	if !strings.Contains(text, "mode: outbound") {
		t.Fatalf("expected outbound network in output: %s", text)
	}
	if !strings.Contains(text, "provider: gemini_openai") {
		t.Fatalf("expected gemini provider in output: %s", text)
	}
	if !strings.Contains(text, "apiKeyEnv: GEMINI_API_KEY") {
		t.Fatalf("expected default GEMINI_API_KEY in output: %s", text)
	}
	if !strings.Contains(text, "target: /config") {
		t.Fatalf("expected /config mount target in output: %s", text)
	}
	if !strings.Contains(text, "target: /logs") {
		t.Fatalf("expected /logs mount target in output: %s", text)
	}
}

func TestRunWizardProviderNoneDisablesLLMBlock(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "obsidian-no-llm.claw")
	vault := filepath.Join(root, "vault")

	code := runWizard([]string{
		"--out", out,
		"--vault", vault,
		"--provider", "none",
	})
	if code != 0 {
		t.Fatalf("runWizard() code = %d, want 0", code)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read generated clawfile: %v", err)
	}
	if strings.Contains(string(b), "\n  llm:\n") {
		t.Fatalf("did not expect llm block when provider=none: %s", string(b))
	}
}

func TestRunWizardRejectsBadRuntime(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "bad-runtime.claw")
	vault := filepath.Join(root, "vault")

	code := runWizard([]string{
		"--out", out,
		"--vault", vault,
		"--runtime", "containerd",
	})
	if code == 0 {
		t.Fatal("expected non-zero exit code for invalid runtime")
	}
}

func TestRunWizardProjectLayout(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")

	code := runWizard([]string{
		"--project-dir", project,
		"--provider", "none",
	})
	if code != 0 {
		t.Fatalf("runWizard() code = %d, want 0", code)
	}
	out := filepath.Join(project, "agent.claw")
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected agent.claw in project dir: %v", err)
	}
	for _, dir := range []string{"vault", "config", "logs"} {
		if _, err := os.Stat(filepath.Join(project, dir)); err != nil {
			t.Fatalf("expected %s dir in project layout: %v", dir, err)
		}
	}
}
