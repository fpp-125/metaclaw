package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	v1 "github.com/metaclaw/metaclaw/internal/claw/schema/v1"
)

var digestRef = regexp.MustCompile(`.+@sha256:[a-fA-F0-9]{64}$`)

func NormalizeAndValidate(cfg v1.Clawfile, clawfilePath string) (v1.Clawfile, error) {
	if err := cfg.ValidateBasics(); err != nil {
		return v1.Clawfile{}, err
	}

	if cfg.Agent.Lifecycle == "" {
		cfg.Agent.Lifecycle = v1.LifecycleEphemeral
	}
	if cfg.Agent.Habitat.Network.Mode == "" {
		cfg.Agent.Habitat.Network.Mode = "none"
	}

	profile, ok := v1.SpeciesProfileFor(cfg.Agent.Species)
	if !ok {
		return v1.Clawfile{}, fmt.Errorf("unknown species: %s", cfg.Agent.Species)
	}
	if cfg.Agent.Runtime.Image == "" {
		cfg.Agent.Runtime.Image = profile.DefaultImage
	}
	if cfg.Agent.Runtime.Resources.CPU == "" {
		cfg.Agent.Runtime.Resources.CPU = profile.DefaultCPU
	}
	if cfg.Agent.Runtime.Resources.Memory == "" {
		cfg.Agent.Runtime.Resources.Memory = profile.DefaultMem
	}
	if len(cfg.Agent.Command) == 0 {
		cfg.Agent.Command = []string{"sh", "-lc", "echo MetaClaw agent started"}
	}

	if !digestRef.MatchString(cfg.Agent.Runtime.Image) {
		return v1.Clawfile{}, fmt.Errorf("agent.runtime.image must be digest-pinned (example: image@sha256:...)")
	}

	if err := validateNetwork(cfg.Agent.Habitat.Network.Mode); err != nil {
		return v1.Clawfile{}, err
	}
	if err := validateMounts(cfg.Agent.Habitat.Mounts); err != nil {
		return v1.Clawfile{}, err
	}
	if err := validateSkills(cfg.Agent.Skills, filepath.Dir(clawfilePath)); err != nil {
		return v1.Clawfile{}, err
	}

	cfg.Agent.Habitat.Env = sortedMap(cfg.Agent.Habitat.Env)
	return cfg, nil
}

func validateNetwork(mode string) error {
	switch mode {
	case "none", "outbound", "all":
		return nil
	default:
		return fmt.Errorf("agent.habitat.network.mode must be one of none,outbound,all")
	}
}

func validateMounts(mounts []v1.MountSpec) error {
	for _, m := range mounts {
		if m.Source == "" || m.Target == "" {
			return fmt.Errorf("every habitat mount requires source and target")
		}
	}
	return nil
}

func validateSkills(skills []v1.SkillRef, baseDir string) error {
	for _, s := range skills {
		hasPath := s.Path != ""
		hasID := s.ID != ""
		if hasPath == hasID {
			return fmt.Errorf("skill entries must specify exactly one of path or id")
		}
		if hasPath {
			resolved := s.Path
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(baseDir, s.Path)
			}
			if _, err := os.Stat(resolved); err != nil {
				return fmt.Errorf("skill path not found: %s", s.Path)
			}
		}
	}
	return nil
}

func sortedMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(in))
	for _, k := range keys {
		out[k] = in[k]
	}
	return out
}
