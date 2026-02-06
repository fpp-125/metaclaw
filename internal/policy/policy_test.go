package policy

import (
	"testing"

	v1 "github.com/metaclaw/metaclaw/internal/claw/schema/v1"
)

func TestCompileDenyByDefaultNetwork(t *testing.T) {
	cfg := v1.Clawfile{
		APIVersion: "metaclaw/v1",
		Kind:       "Agent",
		Agent: v1.AgentSpec{
			Name:      "a",
			Species:   v1.SpeciesNano,
			Lifecycle: v1.LifecycleEphemeral,
			Habitat: v1.HabitatSpec{
				Network: v1.NetworkSpec{Mode: "none"},
			},
		},
	}
	p, err := Compile(cfg)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if p.Network.Allowed {
		t.Fatal("expected network to be denied for mode=none")
	}
}
