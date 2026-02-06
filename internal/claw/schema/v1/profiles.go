package v1

var speciesProfiles = map[Species]SpeciesProfile{
	SpeciesNano: {
		Name:         SpeciesNano,
		DefaultImage: "alpine:3.20@sha256:a4f4213abb84c497377b8544c81b3564f313746700372ec4fe84653e4fb03805",
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
		DefaultImage: "alpine:3.20@sha256:a4f4213abb84c497377b8544c81b3564f313746700372ec4fe84653e4fb03805",
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
		DefaultImage: "ubuntu:24.04@sha256:cd1dba651b3080c3686ecf4e3c4220f026b521fb76978881737d24f200828b2b",
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
