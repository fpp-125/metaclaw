package manager

import "testing"

func TestParseContainerInspectStateArray(t *testing.T) {
	raw := `[{"State":{"Status":"exited","ExitCode":0}}]`
	status, exitCode, err := parseContainerInspectState(raw)
	if err != nil {
		t.Fatalf("parseContainerInspectState() error = %v", err)
	}
	if status != "exited" {
		t.Fatalf("expected exited status, got %q", status)
	}
	if exitCode == nil || *exitCode != 0 {
		t.Fatalf("expected exitCode=0, got %+v", exitCode)
	}
}

func TestParseContainerInspectStateObject(t *testing.T) {
	raw := `{"State":{"Status":"running"}}`
	status, exitCode, err := parseContainerInspectState(raw)
	if err != nil {
		t.Fatalf("parseContainerInspectState() error = %v", err)
	}
	if status != "running" {
		t.Fatalf("expected running status, got %q", status)
	}
	if exitCode != nil {
		t.Fatalf("expected nil exitCode, got %+v", exitCode)
	}
}

func TestParseContainerInspectStateLowercaseFields(t *testing.T) {
	raw := `{"state":{"status":"exited","exitCode":23}}`
	status, exitCode, err := parseContainerInspectState(raw)
	if err != nil {
		t.Fatalf("parseContainerInspectState() error = %v", err)
	}
	if status != "exited" {
		t.Fatalf("expected exited status, got %q", status)
	}
	if exitCode == nil || *exitCode != 23 {
		t.Fatalf("expected exitCode=23, got %+v", exitCode)
	}
}

func TestMapContainerStatus(t *testing.T) {
	exitZero := 0
	status, terminal := mapContainerStatus("exited", &exitZero)
	if !terminal || status != "succeeded" {
		t.Fatalf("expected succeeded terminal state, got status=%q terminal=%v", status, terminal)
	}

	exitNonZero := 17
	status, terminal = mapContainerStatus("exited", &exitNonZero)
	if !terminal || status != "failed" {
		t.Fatalf("expected failed terminal state, got status=%q terminal=%v", status, terminal)
	}

	status, terminal = mapContainerStatus("running", nil)
	if terminal || status != "running" {
		t.Fatalf("expected non-terminal running state, got status=%q terminal=%v", status, terminal)
	}
}
