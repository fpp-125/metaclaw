package validate

import (
	"strings"
	"testing"

	v1 "github.com/metaclaw/metaclaw/internal/claw/schema/v1"
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
