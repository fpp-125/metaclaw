package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/metaclaw/metaclaw/internal/capsule"
	"github.com/metaclaw/metaclaw/internal/compiler"
	"github.com/metaclaw/metaclaw/internal/manager"
)

func Execute(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 1
	}
	ctx := context.Background()
	cmd := args[0]
	switch cmd {
	case "init":
		return runInit(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "compile":
		return runCompile(args[1:])
	case "run":
		return runRun(ctx, args[1:])
	case "ps":
		return runPS(args[1:])
	case "logs":
		return runLogs(ctx, args[1:])
	case "inspect":
		return runInspect(ctx, args[1:])
	case "debug":
		return runDebug(ctx, args[1:])
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		return 1
	}
}

func runInit(args []string) int {
	args = reorderFlags(args, map[string]bool{"--out": true, "-out": true})
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	var out string
	fs.StringVar(&out, "out", "agent.claw", "output path")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	template := `apiVersion: metaclaw/v1
kind: Agent
agent:
  name: hello-agent
  species: nano
  lifecycle: ephemeral
  habitat:
    network:
      mode: none
    mounts: []
    env: {}
  runtime:
    # Optional; resolved by species if omitted
    # image: alpine:3.20@sha256:77726ef25f24bcc9d8e059309a8929574b2f13f0707cde656d2d7b82f83049c4
  command:
    - sh
    - -lc
    - echo "Hello from MetaClaw"
`
	if err := os.WriteFile(out, []byte(template), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write template: %v\n", err)
		return 1
	}
	fmt.Printf("created %s\n", out)
	return 0
}

func runValidate(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: metaclaw validate <file.claw>")
		return 1
	}
	cfg, err := compiler.LoadNormalize(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate failed: %v\n", err)
		return 1
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Println(string(b))
	fmt.Println("validation: OK")
	return 0
}

func runCompile(args []string) int {
	args = reorderFlags(args, map[string]bool{"-o": true})
	fs := flag.NewFlagSet("compile", flag.ContinueOnError)
	var out string
	fs.StringVar(&out, "o", ".", "output directory")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	remaining := fs.Args()
	if len(remaining) != 1 {
		fmt.Fprintln(os.Stderr, "usage: metaclaw compile <file.claw> [-o dir]")
		return 1
	}
	res, err := compiler.Compile(remaining[0], out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compile failed: %v\n", err)
		return 1
	}
	fmt.Printf("capsule: %s\n", res.Capsule.Path)
	fmt.Printf("capsule_id: %s\n", res.Capsule.ID)
	return 0
}

func runRun(ctx context.Context, args []string) int {
	if err := IsSecurityOverrideFlag(args); err != nil {
		fmt.Fprintf(os.Stderr, "run blocked: %v\n", err)
		return 1
	}
	args = reorderFlags(args, map[string]bool{"--runtime": true, "--state-dir": true})
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	var detach bool
	var runtimeOverride string
	var stateDir string
	fs.BoolVar(&detach, "detach", false, "run in background")
	fs.StringVar(&runtimeOverride, "runtime", "", "runtime override (podman|apple_container|docker)")
	fs.StringVar(&stateDir, "state-dir", ".metaclaw", "state directory")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	remaining := fs.Args()
	if len(remaining) != 1 {
		fmt.Fprintln(os.Stderr, "usage: metaclaw run <file.claw|capsule_dir> [--detach] [--runtime=..] [--state-dir=.metaclaw]")
		return 1
	}
	m, err := manager.New(stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open manager: %v\n", err)
		return 1
	}
	defer m.Close()

	r, err := m.Run(ctx, manager.RunOptions{InputPath: remaining[0], Detach: detach, RuntimeOverride: runtimeOverride})
	if err != nil {
		fmt.Fprintf(os.Stderr, "run failed: %v\n", err)
		fmt.Printf("run_id: %s\n", r.RunID)
		fmt.Printf("status: %s\n", r.Status)
		return 1
	}
	fmt.Printf("run_id: %s\n", r.RunID)
	fmt.Printf("status: %s\n", r.Status)
	fmt.Printf("runtime: %s\n", r.RuntimeTarget)
	fmt.Printf("container: %s\n", r.ContainerID)
	return 0
}

func runPS(args []string) int {
	args = reorderFlags(args, map[string]bool{"--state-dir": true, "--limit": true})
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	var stateDir string
	var limit int
	var asJSON bool
	fs.StringVar(&stateDir, "state-dir", ".metaclaw", "state directory")
	fs.IntVar(&limit, "limit", 50, "max rows")
	fs.BoolVar(&asJSON, "json", false, "json output")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	m, err := manager.New(stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open manager: %v\n", err)
		return 1
	}
	defer m.Close()
	runs, err := m.ListRuns(limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ps failed: %v\n", err)
		return 1
	}
	if asJSON {
		b, _ := json.MarshalIndent(runs, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	for _, r := range runs {
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", r.RunID, r.Status, r.RuntimeTarget, r.Lifecycle, r.CapsuleID)
	}
	return 0
}

func runLogs(ctx context.Context, args []string) int {
	args = reorderFlags(args, map[string]bool{"--state-dir": true})
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	var stateDir string
	var follow bool
	fs.StringVar(&stateDir, "state-dir", ".metaclaw", "state directory")
	fs.BoolVar(&follow, "follow", false, "follow runtime logs")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	remaining := fs.Args()
	if len(remaining) != 1 {
		fmt.Fprintln(os.Stderr, "usage: metaclaw logs <run-id> [--follow]")
		return 1
	}
	runID := remaining[0]
	m, err := manager.New(stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open manager: %v\n", err)
		return 1
	}
	defer m.Close()

	events, err := m.ReadEvents(runID)
	if err == nil {
		for _, line := range events {
			fmt.Println(line)
		}
	}

	r, err := m.GetRun(runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run not found: %v\n", err)
		return 1
	}
	logsText, err := m.RuntimeLogs(ctx, r, follow)
	if err == nil && strings.TrimSpace(logsText) != "" {
		fmt.Print(logsText)
	}
	stdoutPath := filepath.Join(stateDir, "runs", runID, "stdout.log")
	stderrPath := filepath.Join(stateDir, "runs", runID, "stderr.log")
	if b, err := os.ReadFile(stdoutPath); err == nil && len(b) > 0 {
		fmt.Print(string(b))
	}
	if b, err := os.ReadFile(stderrPath); err == nil && len(b) > 0 {
		fmt.Print(string(b))
	}
	return 0
}

func runInspect(ctx context.Context, args []string) int {
	args = reorderFlags(args, map[string]bool{"--state-dir": true})
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	var stateDir string
	var asJSON bool
	fs.StringVar(&stateDir, "state-dir", ".metaclaw", "state directory")
	fs.BoolVar(&asJSON, "json", false, "json output")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	remaining := fs.Args()
	if len(remaining) != 1 {
		fmt.Fprintln(os.Stderr, "usage: metaclaw inspect <run-id|capsule-dir> [--json]")
		return 1
	}
	target := remaining[0]
	if st, err := os.Stat(target); err == nil && st.IsDir() {
		m, err := capsule.Load(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "inspect capsule failed: %v\n", err)
			return 1
		}
		if asJSON {
			b, _ := json.MarshalIndent(m, "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Printf("capsule_id: %s\n", m.CapsuleID)
			fmt.Printf("source: %s\n", m.SourceClawfile)
			fmt.Printf("digests: %d entries\n", len(m.Digests))
		}
		return 0
	}
	m, err := manager.New(stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open manager: %v\n", err)
		return 1
	}
	defer m.Close()
	r, err := m.GetRun(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inspect run failed: %v\n", err)
		return 1
	}
	rt, inspectErr := m.RuntimeInspect(ctx, r)
	payload := map[string]any{"run": r, "runtimeInspect": rt}
	if inspectErr != nil {
		payload["runtimeInspectError"] = inspectErr.Error()
	}
	if asJSON {
		b, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	fmt.Printf("run_id: %s\n", r.RunID)
	fmt.Printf("status: %s\n", r.Status)
	fmt.Printf("runtime: %s\n", r.RuntimeTarget)
	fmt.Printf("container: %s\n", r.ContainerID)
	if inspectErr != nil {
		fmt.Printf("runtime inspect error: %v\n", inspectErr)
	}
	return 0
}

func runDebug(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] != "shell" {
		fmt.Fprintln(os.Stderr, "usage: metaclaw debug shell <run-id> [--state-dir=.metaclaw]")
		return 1
	}
	parsed := reorderFlags(args[1:], map[string]bool{"--state-dir": true})
	fs := flag.NewFlagSet("debug shell", flag.ContinueOnError)
	var stateDir string
	fs.StringVar(&stateDir, "state-dir", ".metaclaw", "state directory")
	if err := fs.Parse(parsed); err != nil {
		return 1
	}
	remaining := fs.Args()
	if len(remaining) != 1 {
		fmt.Fprintln(os.Stderr, "usage: metaclaw debug shell <run-id> [--state-dir=.metaclaw]")
		return 1
	}
	m, err := manager.New(stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open manager: %v\n", err)
		return 1
	}
	defer m.Close()
	if err := m.DebugShell(ctx, remaining[0]); err != nil {
		fmt.Fprintf(os.Stderr, "debug shell failed: %v\n", err)
		return 1
	}
	return 0
}

func reorderFlags(args []string, valueFlags map[string]bool) []string {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			if takesValue(a, valueFlags) && !strings.Contains(a, "=") && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positionals = append(positionals, a)
	}
	return append(flags, positionals...)
}

func takesValue(flagToken string, valueFlags map[string]bool) bool {
	if valueFlags[flagToken] {
		return true
	}
	if eq := strings.Index(flagToken, "="); eq > 0 {
		return valueFlags[flagToken[:eq]]
	}
	return false
}

func printUsage() {
	fmt.Print(`metaclaw - local-first infrastructure engine for AI agents

commands:
  init
  validate <file.claw>
  compile <file.claw> [-o dir]
  run <file.claw|capsule_dir> [--detach] [--runtime=podman|apple_container|docker]
  ps [--json]
  logs <run-id> [--follow]
  inspect <run-id|capsule-dir> [--json]
  debug shell <run-id>
`)
}

func IsSecurityOverrideFlag(args []string) error {
	for _, a := range args {
		if strings.HasPrefix(a, "--mount") || strings.HasPrefix(a, "--network") || strings.HasPrefix(a, "--env") {
			return errors.New("CLI overrides for habitat security boundaries are not allowed")
		}
	}
	return nil
}
