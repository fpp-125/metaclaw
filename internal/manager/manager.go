package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/metaclaw/metaclaw/internal/capsule"
	v1 "github.com/metaclaw/metaclaw/internal/claw/schema/v1"
	"github.com/metaclaw/metaclaw/internal/compiler"
	"github.com/metaclaw/metaclaw/internal/llm"
	"github.com/metaclaw/metaclaw/internal/logs"
	"github.com/metaclaw/metaclaw/internal/policy"
	"github.com/metaclaw/metaclaw/internal/runtime"
	"github.com/metaclaw/metaclaw/internal/runtime/spec"
	store "github.com/metaclaw/metaclaw/internal/store/sqlite"
)

type Manager struct {
	stateDir string
	store    *store.Store
	resolver *runtime.Resolver
}

type RunOptions struct {
	InputPath       string
	Detach          bool
	RuntimeOverride string
	LLMAPIKey       string
	LLMAPIKeyEnv    string
}

type RunOutcome struct {
	Run   store.RunRecord
	Error error
}

func New(stateDir string) (*Manager, error) {
	if stateDir == "" {
		stateDir = ".metaclaw"
	}
	s, err := store.Open(stateDir)
	if err != nil {
		return nil, err
	}
	return &Manager{stateDir: stateDir, store: s, resolver: runtime.NewResolver()}, nil
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	return m.store.Close()
}

func (m *Manager) Run(ctx context.Context, opts RunOptions) (store.RunRecord, error) {
	cfg, pol, capPath, capID, err := m.prepareCapsule(opts.InputPath)
	if err != nil {
		return store.RunRecord{}, err
	}
	if err := m.store.UpsertCapsule(capID, capPath); err != nil {
		return store.RunRecord{}, err
	}

	adapter, target, err := m.resolver.Resolve(ctx, opts.RuntimeOverride, string(cfg.Agent.Runtime.Target))
	if err != nil {
		return store.RunRecord{}, err
	}
	resolvedLLM, err := llm.Resolve(cfg.Agent.LLM, llm.RuntimeOptions{
		APIKey:    opts.LLMAPIKey,
		APIKeyEnv: opts.LLMAPIKeyEnv,
	})
	if err != nil {
		return store.RunRecord{}, err
	}
	env := mergeEnv(cfg.Agent.Habitat.Env, resolvedLLM.Env)

	runID := makeRunID()
	rec := store.RunRecord{
		RunID:         runID,
		CapsuleID:     capID,
		CapsulePath:   capPath,
		Status:        "running",
		Lifecycle:     string(cfg.Agent.Lifecycle),
		RuntimeTarget: string(target),
		StartedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := m.store.InsertRun(rec); err != nil {
		return store.RunRecord{}, err
	}
	_ = logs.AppendEvent(m.stateDir, runID, logs.Event{Phase: "runtime.resolve", Runtime: string(target), Message: "runtime selected"})

	containerName := "metaclaw_" + runID
	runRes, runErr := adapter.Run(ctx, spec.RunOptions{
		ContainerName: containerName,
		Image:         cfg.Agent.Runtime.Image,
		Command:       cfg.Agent.Command,
		Detach:        opts.Detach || cfg.Agent.Lifecycle == v1.LifecycleDaemon,
		Policy:        pol,
		Env:           env,
		Workdir:       cfg.Agent.Habitat.Workdir,
		User:          cfg.Agent.Habitat.User,
		CPU:           cfg.Agent.Runtime.Resources.CPU,
		Memory:        cfg.Agent.Runtime.Resources.Memory,
	})

	containerID := runRes.ContainerID
	if containerID == "" {
		containerID = containerName
	}
	rec.ContainerID = containerID
	_ = writeRunOutput(m.stateDir, runID, "stdout.log", runRes.Stdout)
	_ = writeRunOutput(m.stateDir, runID, "stderr.log", runRes.Stderr)

	detached := opts.Detach || cfg.Agent.Lifecycle == v1.LifecycleDaemon
	if detached {
		if runErr != nil {
			errText := runErr.Error()
			_ = logs.AppendEvent(m.stateDir, runID, logs.Event{Phase: "runtime.start", Runtime: string(target), ContainerID: containerID, Message: "daemon start failed", Error: errText})
			_ = m.store.UpdateRunCompletion(runID, "failed", containerID, intPtr(runRes.ExitCode), errText)
			rec.Status = "failed"
			rec.LastError = errText
			rec.ExitCode = intPtr(runRes.ExitCode)
			return rec, runErr
		}
		_ = logs.AppendEvent(m.stateDir, runID, logs.Event{Phase: "runtime.start", Runtime: string(target), ContainerID: containerID, Message: "daemon started"})
		_ = m.store.UpdateRunStatus(runID, "running", containerID, "")
		rec.Status = "running"
		rec.ContainerID = containerID
		return rec, nil
	}

	status := "succeeded"
	var lastError string
	exitPtr := intPtr(runRes.ExitCode)
	if runErr != nil || runRes.ExitCode != 0 {
		status = "failed"
		if runErr != nil {
			lastError = runErr.Error()
		}
	}

	if status == "failed" && cfg.Agent.Lifecycle == v1.LifecycleDebug {
		status = "failed_paused"
		_ = logs.AppendEvent(m.stateDir, runID, logs.Event{Phase: "runtime.pause", Runtime: string(target), ContainerID: containerID, Message: "container preserved for debug", Error: lastError})
	} else {
		if remErr := adapter.Remove(ctx, containerID); remErr == nil {
			_ = logs.AppendEvent(m.stateDir, runID, logs.Event{Phase: "runtime.cleanup", Runtime: string(target), ContainerID: containerID, Message: "container removed"})
		}
	}

	_ = m.store.UpdateRunCompletion(runID, status, containerID, exitPtr, lastError)
	rec.Status = status
	rec.ExitCode = exitPtr
	rec.LastError = lastError
	rec.EndedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if status == "succeeded" {
		_ = logs.AppendEvent(m.stateDir, runID, logs.Event{Phase: "runtime.exit", Runtime: string(target), ContainerID: containerID, Message: "completed"})
		return rec, nil
	}
	_ = logs.AppendEvent(m.stateDir, runID, logs.Event{Phase: "runtime.exit", Runtime: string(target), ContainerID: containerID, Message: "failed", Error: lastError})
	if runErr != nil {
		return rec, runErr
	}
	return rec, fmt.Errorf("run failed with exit code %d", runRes.ExitCode)
}

func (m *Manager) ListRuns(limit int) ([]store.RunRecord, error) {
	return m.store.ListRuns(limit)
}

func (m *Manager) GetRun(runID string) (store.RunRecord, error) {
	return m.store.GetRun(runID)
}

func (m *Manager) ReadEvents(runID string) ([]string, error) {
	return logs.ReadEvents(m.stateDir, runID)
}

func (m *Manager) RuntimeLogs(ctx context.Context, r store.RunRecord, follow bool) (string, error) {
	t, err := runtime.ParseTarget(r.RuntimeTarget)
	if err != nil {
		return "", err
	}
	ad, ok := m.resolver.Adapter(t)
	if !ok {
		return "", fmt.Errorf("runtime adapter unavailable: %s", r.RuntimeTarget)
	}
	return ad.Logs(ctx, r.ContainerID, follow)
}

func (m *Manager) RuntimeInspect(ctx context.Context, r store.RunRecord) (string, error) {
	t, err := runtime.ParseTarget(r.RuntimeTarget)
	if err != nil {
		return "", err
	}
	ad, ok := m.resolver.Adapter(t)
	if !ok {
		return "", fmt.Errorf("runtime adapter unavailable: %s", r.RuntimeTarget)
	}
	return ad.Inspect(ctx, r.ContainerID)
}

func (m *Manager) DebugShell(ctx context.Context, runID string) error {
	r, err := m.store.GetRun(runID)
	if err != nil {
		return err
	}
	if r.Status != "failed_paused" && r.Status != "running" {
		return fmt.Errorf("run %s is not debuggable (status=%s)", runID, r.Status)
	}
	t, err := runtime.ParseTarget(r.RuntimeTarget)
	if err != nil {
		return err
	}
	ad, ok := m.resolver.Adapter(t)
	if !ok {
		return fmt.Errorf("runtime adapter unavailable: %s", r.RuntimeTarget)
	}
	return ad.ExecShell(ctx, r.ContainerID)
}

func (m *Manager) prepareCapsule(inputPath string) (v1.Clawfile, policy.Policy, string, string, error) {
	st, err := os.Stat(inputPath)
	if err != nil {
		return v1.Clawfile{}, policy.Policy{}, "", "", err
	}
	if !st.IsDir() && strings.HasSuffix(inputPath, ".claw") {
		outDir := filepath.Join(m.stateDir, "capsules")
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return v1.Clawfile{}, policy.Policy{}, "", "", err
		}
		res, err := compiler.Compile(inputPath, outDir)
		if err != nil {
			return v1.Clawfile{}, policy.Policy{}, "", "", err
		}
		return res.Config, res.Policy, res.Capsule.Path, res.Capsule.ID, nil
	}
	if st.IsDir() {
		return loadFromCapsuleDir(inputPath)
	}
	return v1.Clawfile{}, policy.Policy{}, "", "", fmt.Errorf("input must be .claw file or capsule directory")
}

func loadFromCapsuleDir(capPath string) (v1.Clawfile, policy.Policy, string, string, error) {
	m, err := capsule.Load(capPath)
	if err != nil {
		return v1.Clawfile{}, policy.Policy{}, "", "", fmt.Errorf("load capsule manifest: %w", err)
	}
	irBytes, err := os.ReadFile(filepath.Join(capPath, "ir.json"))
	if err != nil {
		return v1.Clawfile{}, policy.Policy{}, "", "", err
	}
	var ir struct {
		Clawfile v1.Clawfile `json:"clawfile"`
	}
	if err := json.Unmarshal(irBytes, &ir); err != nil {
		return v1.Clawfile{}, policy.Policy{}, "", "", fmt.Errorf("parse capsule ir: %w", err)
	}
	pBytes, err := os.ReadFile(filepath.Join(capPath, "policy.json"))
	if err != nil {
		return v1.Clawfile{}, policy.Policy{}, "", "", err
	}
	var pol policy.Policy
	if err := json.Unmarshal(pBytes, &pol); err != nil {
		return v1.Clawfile{}, policy.Policy{}, "", "", fmt.Errorf("parse capsule policy: %w", err)
	}
	return ir.Clawfile, pol, capPath, m.CapsuleID, nil
}

func makeRunID() string {
	now := time.Now().UTC()
	return now.Format("20060102t150405") + fmt.Sprintf("%09d", now.Nanosecond())
}

func writeRunOutput(stateDir, runID, fileName, content string) error {
	path := filepath.Join(stateDir, "runs", runID, fileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func intPtr(v int) *int { return &v }

func mergeEnv(base map[string]string, overlay map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}
