package v1

var speciesProfiles = map[Species]SpeciesProfile{
	SpeciesNano: {
		Name:         SpeciesNano,
		DefaultImage: "alpine:3.20@sha256:77726ef25f24bcc9d8e059309a8929574b2f13f0707cde656d2d7b82f83049c4",
		DefaultCPU:   "0.25",
		DefaultMem:   "256m",
		RuntimeHints: []string{"wasm_preferred"},
		Allowed: AllowedPatch{
			AllowResourceOverride: true,
			AllowImageOverride:    true,
		},
	},
	SpeciesMicro: {
		Name:         SpeciesMicro,
		DefaultImage: "alpine:3.20@sha256:77726ef25f24bcc9d8e059309a8929574b2f13f0707cde656d2d7b82f83049c4",
		DefaultCPU:   "1",
		DefaultMem:   "512m",
		RuntimeHints: []string{"container_default"},
		Allowed: AllowedPatch{
			AllowResourceOverride: true,
			AllowImageOverride:    true,
		},
	},
	SpeciesMega: {
		Name:         SpeciesMega,
		DefaultImage: "ubuntu:24.04@sha256:c35e29c9450151419d9448b0fd75374fec4fff364a27f176fb458d472dfc9e54",
		DefaultCPU:   "2",
		DefaultMem:   "2g",
		RuntimeHints: []string{"container_heavy"},
		Allowed: AllowedPatch{
			AllowResourceOverride: true,
			AllowImageOverride:    true,
		},
	},
}

func SpeciesProfileFor(s Species) (SpeciesProfile, bool) {
	p, ok := speciesProfiles[s]
	return p, ok
}
