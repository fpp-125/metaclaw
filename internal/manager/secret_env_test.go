package manager

import (
	"os"
	"testing"
)

func TestResolveHostSecretEnvs(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "tvly-dev-example")
	t.Setenv("ANOTHER_SECRET", "value-2")
	got, err := resolveHostSecretEnvs([]string{"TAVILY_API_KEY", "ANOTHER_SECRET"})
	if err != nil {
		t.Fatalf("resolveHostSecretEnvs error: %v", err)
	}
	if got["TAVILY_API_KEY"] != "tvly-dev-example" {
		t.Fatalf("unexpected TAVILY_API_KEY: %q", got["TAVILY_API_KEY"])
	}
	if got["ANOTHER_SECRET"] != "value-2" {
		t.Fatalf("unexpected ANOTHER_SECRET: %q", got["ANOTHER_SECRET"])
	}
}

func TestResolveHostSecretEnvsRejectsInvalidName(t *testing.T) {
	_, err := resolveHostSecretEnvs([]string{"BAD-NAME"})
	if err == nil {
		t.Fatal("expected invalid env name error")
	}
}

func TestResolveHostSecretEnvsRejectsMissingValue(t *testing.T) {
	_ = os.Unsetenv("MISSING_SECRET")
	_, err := resolveHostSecretEnvs([]string{"MISSING_SECRET"})
	if err == nil {
		t.Fatal("expected missing env error")
	}
}

func TestMergeEnvMany(t *testing.T) {
	out := mergeEnvMany(
		map[string]string{"A": "1", "B": "2"},
		map[string]string{"B": "override", "C": "3"},
	)
	if out["A"] != "1" || out["B"] != "override" || out["C"] != "3" {
		t.Fatalf("unexpected merged env: %+v", out)
	}
}
