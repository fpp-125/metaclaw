package validate

import (
	"strings"
	"testing"

	v1 "github.com/fpp-125/metaclaw/internal/claw/schema/v1"
)

func TestNormalizeDefaults(t *testing.T) {
	cfg := v1.Clawfile{
		APIVersion: "metaclaw/v1",
		Kind:       "Agent",
		Agent: v1.AgentSpec{
			Name:    "a",
			Species: v1.SpeciesNano,
		},
	}
	got, err := NormalizeAndValidate(cfg, "agent.claw")
	if err != nil {
		t.Fatalf("NormalizeAndValidate() error = %v", err)
	}
	if got.Agent.Lifecycle != v1.LifecycleEphemeral {
		t.Fatalf("expected default lifecycle ephemeral, got %q", got.Agent.Lifecycle)
	}
	if got.Agent.Habitat.Network.Mode != "none" {
		t.Fatalf("expected default network none, got %q", got.Agent.Habitat.Network.Mode)
	}
	if got.Agent.Runtime.Image == "" || !strings.Contains(got.Agent.Runtime.Image, "@sha256:") {
		t.Fatalf("expected digest-pinned default image, got %q", got.Agent.Runtime.Image)
	}
}

func TestRejectUnpinnedImage(t *testing.T) {
	cfg := v1.Clawfile{
		APIVersion: "metaclaw/v1",
		Kind:       "Agent",
		Agent: v1.AgentSpec{
			Name:    "a",
			Species: v1.SpeciesNano,
			Runtime: v1.RuntimeSpec{Image: "alpine:latest"},
		},
	}
	_, err := NormalizeAndValidate(cfg, "agent.claw")
	if err == nil {
		t.Fatal("expected validation error for unpinned image")
	}
}

func TestNormalizeLLMGeminiDefaults(t *testing.T) {
	cfg := v1.Clawfile{
		APIVersion: "metaclaw/v1",
		Kind:       "Agent",
		Agent: v1.AgentSpec{
			Name:    "a",
			Species: v1.SpeciesNano,
			LLM: v1.LLMSpec{
				Provider: v1.LLMProviderGeminiOpenAI,
				Model:    "gemini-2.5-pro",
			},
		},
	}
	got, err := NormalizeAndValidate(cfg, "agent.claw")
	if err != nil {
		t.Fatalf("NormalizeAndValidate() error = %v", err)
	}
	if got.Agent.LLM.APIKeyEnv != "GEMINI_API_KEY" {
		t.Fatalf("expected default apiKeyEnv GEMINI_API_KEY, got %q", got.Agent.LLM.APIKeyEnv)
	}
	if got.Agent.LLM.BaseURL == "" {
		t.Fatal("expected default Gemini OpenAI-compatible baseURL")
	}
}

func TestRejectLLMWithoutProvider(t *testing.T) {
	cfg := v1.Clawfile{
		APIVersion: "metaclaw/v1",
		Kind:       "Agent",
		Agent: v1.AgentSpec{
			Name:    "a",
			Species: v1.SpeciesNano,
			LLM: v1.LLMSpec{
				Model: "gemini-2.5-pro",
			},
		},
	}
	_, err := NormalizeAndValidate(cfg, "agent.claw")
	if err == nil {
		t.Fatal("expected validation error when llm provider is missing")
	}
}

func TestRejectLLMWithoutModel(t *testing.T) {
	cfg := v1.Clawfile{
		APIVersion: "metaclaw/v1",
		Kind:       "Agent",
		Agent: v1.AgentSpec{
			Name:    "a",
			Species: v1.SpeciesNano,
			LLM: v1.LLMSpec{
				Provider: v1.LLMProviderOpenAICompatible,
			},
		},
	}
	_, err := NormalizeAndValidate(cfg, "agent.claw")
	if err == nil {
		t.Fatal("expected validation error when llm model is missing")
	}
}
