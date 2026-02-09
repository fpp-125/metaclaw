package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/term"
)

type onboardOptions struct {
	ProjectDir string
	VaultPath  string
	Runtime    string
	Profile    string
	LLMKeyEnv  string
	WebKeyEnv  string

	SkipBuild bool
	NoRun     bool
	Force     bool

	InteractiveExplicit bool
	SaveEnv             bool
}

func runOnboard(args []string) int {
	rawArgs := append([]string(nil), args...)

	args = reorderFlags(args, map[string]bool{
		"--project-dir": true,
		"--vault":       true,
		"--runtime":     true,
		"--profile":     true,
		"--llm-key-env": true,
		"--web-key-env": true,
		"--interactive": false,
		"--save-env":    false,
		"--skip-build":  false,
		"--no-run":      false,
		"--force":       false,
	})

	fs := flag.NewFlagSet("onboard", flag.ContinueOnError)
	opts := onboardOptions{
		ProjectDir: "./my-obsidian-bot",
		Runtime:    "auto",
		Profile:    "obsidian-chat",
		LLMKeyEnv:  "OPENAI_FORMAT_API_KEY",
		WebKeyEnv:  "TAVILY_API_KEY",
		SaveEnv:    true,
	}
	fs.StringVar(&opts.ProjectDir, "project-dir", opts.ProjectDir, "project directory (default ./my-obsidian-bot)")
	fs.StringVar(&opts.VaultPath, "vault", "", "absolute vault path (interactive prompt if omitted)")
	fs.StringVar(&opts.Runtime, "runtime", opts.Runtime, "runtime target (auto|apple_container|podman|docker)")
	fs.StringVar(&opts.Profile, "profile", opts.Profile, "profile (obsidian-chat|obsidian-research)")
	fs.StringVar(&opts.LLMKeyEnv, "llm-key-env", opts.LLMKeyEnv, "LLM API key env name (default OPENAI_FORMAT_API_KEY)")
	fs.StringVar(&opts.WebKeyEnv, "web-key-env", opts.WebKeyEnv, "web search API key env name (default TAVILY_API_KEY)")
	fs.BoolVar(&opts.InteractiveExplicit, "interactive", false, "run interactive step-by-step onboarding")
	fs.BoolVar(&opts.SaveEnv, "save-env", opts.SaveEnv, "write keys into <project>/.env for convenience (gitignored)")
	fs.BoolVar(&opts.SkipBuild, "skip-build", false, "skip image build")
	fs.BoolVar(&opts.NoRun, "no-run", false, "prepare project only, do not launch chat")
	fs.BoolVar(&opts.Force, "force", false, "allow using a non-empty project directory")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	remaining := fs.Args()
	if len(remaining) != 1 || remaining[0] != "obsidian" {
		fmt.Fprintln(os.Stderr, "usage: metaclaw onboard obsidian [--interactive] [--project-dir=./my-obsidian-bot] [--vault=/abs/path/to/vault] [--runtime=auto|apple_container|podman|docker] [--profile=obsidian-chat] [--save-env] [--skip-build] [--no-run] [--force]")
		return 1
	}

	modeInteractive := opts.InteractiveExplicit || (len(rawArgs) == 1 && rawArgs[0] == "obsidian")
	if modeInteractive {
		if !isInteractiveTerminal() {
			fmt.Fprintln(os.Stderr, "onboard failed: interactive prompts require a TTY (pass flags instead)")
			return 1
		}
		var err error
		opts, err = collectOnboardInteractiveOptions(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "onboard failed: %v\n", err)
			return 1
		}
	} else {
		if strings.TrimSpace(opts.VaultPath) == "" {
			fmt.Fprintln(os.Stderr, "onboard failed: --vault is required in non-interactive mode")
			return 1
		}
	}

	opts.LLMKeyEnv = strings.TrimSpace(opts.LLMKeyEnv)
	if opts.LLMKeyEnv == "" {
		opts.LLMKeyEnv = "OPENAI_FORMAT_API_KEY"
	}
	if !wizardEnvNameRef.MatchString(opts.LLMKeyEnv) {
		fmt.Fprintln(os.Stderr, "onboard failed: --llm-key-env must be a valid environment variable name")
		return 1
	}
	opts.WebKeyEnv = strings.TrimSpace(opts.WebKeyEnv)
	if opts.WebKeyEnv == "" {
		opts.WebKeyEnv = "TAVILY_API_KEY"
	}
	if !wizardEnvNameRef.MatchString(opts.WebKeyEnv) {
		fmt.Fprintln(os.Stderr, "onboard failed: --web-key-env must be a valid environment variable name")
		return 1
	}

	var err error
	opts.ProjectDir, err = filepath.Abs(strings.TrimSpace(opts.ProjectDir))
	if err != nil {
		fmt.Fprintf(os.Stderr, "onboard failed: resolve project dir: %v\n", err)
		return 1
	}
	opts.VaultPath, err = filepath.Abs(strings.TrimSpace(opts.VaultPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "onboard failed: resolve vault path: %v\n", err)
		return 1
	}

	// Ensure an LLM key exists (either already in env or entered interactively).
	if strings.TrimSpace(os.Getenv(opts.LLMKeyEnv)) == "" {
		if modeInteractive {
			key, err := promptSecret(os.Stderr, fmt.Sprintf("Enter %s (hidden input): ", opts.LLMKeyEnv))
			if err != nil {
				fmt.Fprintf(os.Stderr, "onboard failed: read key: %v\n", err)
				return 1
			}
			key = strings.TrimSpace(key)
			if key == "" {
				fmt.Fprintf(os.Stderr, "onboard failed: %s cannot be empty\n", opts.LLMKeyEnv)
				return 1
			}
			_ = os.Setenv(opts.LLMKeyEnv, key)
		} else {
			fmt.Fprintf(os.Stderr, "onboard failed: missing LLM key (export %s=...)\n", opts.LLMKeyEnv)
			return 1
		}
	}

	// Prepare project via quickstart (always --no-run so we can optionally write .env first).
	quickArgs := []string{
		"obsidian",
		"--project-dir", opts.ProjectDir,
		"--vault", opts.VaultPath,
		"--runtime", strings.TrimSpace(opts.Runtime),
		"--profile", strings.TrimSpace(opts.Profile),
		"--llm-key-env", opts.LLMKeyEnv,
		"--web-key-env", opts.WebKeyEnv,
		"--no-run",
	}
	if opts.SkipBuild {
		quickArgs = append(quickArgs, "--skip-build")
	}
	if opts.Force {
		quickArgs = append(quickArgs, "--force")
	}
	if rc := runQuickstart(quickArgs); rc != 0 {
		return rc
	}

	if opts.SaveEnv {
		env := map[string]string{}
		env[opts.LLMKeyEnv] = strings.TrimSpace(os.Getenv(opts.LLMKeyEnv))
		if strings.TrimSpace(os.Getenv(opts.WebKeyEnv)) != "" {
			env[opts.WebKeyEnv] = strings.TrimSpace(os.Getenv(opts.WebKeyEnv))
		}
		if err := writeDotEnvFile(filepath.Join(opts.ProjectDir, ".env"), env); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write .env: %v\n", err)
		}
	}

	if opts.NoRun {
		return 0
	}

	exePath := "metaclaw"
	if exe, err := os.Executable(); err == nil {
		exePath = exe
	}
	fmt.Println("launching chat...")
	if err := runScript(filepath.Join(opts.ProjectDir, "chat.sh"), opts.ProjectDir, map[string]string{
		"METACLAW_BIN": exePath,
	}, true); err != nil {
		fmt.Fprintf(os.Stderr, "onboard failed: chat.sh: %v\n", err)
		return 1
	}
	return 0
}

func collectOnboardInteractiveOptions(in onboardOptions) (onboardOptions, error) {
	reader := bufio.NewReader(os.Stdin)

	project, err := promptLine(reader, os.Stderr, "Project directory", in.ProjectDir)
	if err != nil {
		return in, err
	}
	in.ProjectDir = project

	vault := strings.TrimSpace(in.VaultPath)
	if vault == "" {
		vault, err = promptLine(reader, os.Stderr, "Obsidian vault path", "")
		if err != nil {
			return in, err
		}
	}
	in.VaultPath = vault

	runtimeHelp := "Runtime target (auto|apple_container|podman|docker)"
	runtime, err := promptLine(reader, os.Stderr, runtimeHelp, in.Runtime)
	if err != nil {
		return in, err
	}
	in.Runtime = runtime

	profileHelp := "Profile (obsidian-chat: offline default, limited scope | obsidian-research: outbound net, full scope)"
	profile, err := promptLine(reader, os.Stderr, profileHelp, in.Profile)
	if err != nil {
		return in, err
	}
	in.Profile = profile

	saveEnv, err := promptYesNo(reader, os.Stderr, "Save keys you enter today into <project>/.env (gitignored) so you don't need to export every time?", in.SaveEnv)
	if err != nil {
		return in, err
	}
	in.SaveEnv = saveEnv

	needWeb, err := promptYesNo(reader, os.Stderr, "Enable web search (optional, requires key)?", strings.TrimSpace(in.Profile) == "obsidian-research")
	if err != nil {
		return in, err
	}
	if needWeb && strings.TrimSpace(os.Getenv(in.WebKeyEnv)) == "" {
		key, err := promptSecret(os.Stderr, fmt.Sprintf("Enter %s (hidden input): ", in.WebKeyEnv))
		if err != nil {
			return in, err
		}
		key = strings.TrimSpace(key)
		if key != "" {
			_ = os.Setenv(in.WebKeyEnv, key)
		}
	}

	launch, err := promptYesNo(reader, os.Stderr, "Launch chat now?", !in.NoRun)
	if err != nil {
		return in, err
	}
	in.NoRun = !launch

	return in, nil
}

func promptLine(r *bufio.Reader, w *os.File, label, defaultValue string) (string, error) {
	for {
		if strings.TrimSpace(defaultValue) != "" {
			fmt.Fprintf(w, "%s [%s]: ", label, defaultValue)
		} else {
			fmt.Fprintf(w, "%s: ", label)
		}
		line, err := r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		value := strings.TrimSpace(line)
		if value == "" {
			value = strings.TrimSpace(defaultValue)
		}
		value = stripOuterQuotes(value)
		if value != "" {
			return value, nil
		}
		if errors.Is(err, io.EOF) {
			return "", errors.New("input closed before value was provided")
		}
		fmt.Fprintln(w, "value is required")
	}
}

func stripOuterQuotes(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"') {
			return strings.TrimSpace(value[1 : len(value)-1])
		}
	}
	return value
}

func promptYesNo(r *bufio.Reader, w *os.File, label string, defaultYes bool) (bool, error) {
	def := "y/N"
	if defaultYes {
		def = "Y/n"
	}
	for {
		fmt.Fprintf(w, "%s [%s]: ", label, def)
		line, err := r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		}
		v := strings.TrimSpace(strings.ToLower(line))
		switch v {
		case "":
			return defaultYes, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(w, "please enter y or n")
		}
		if errors.Is(err, io.EOF) {
			return false, errors.New("input closed before selection was provided")
		}
	}
}

func promptSecret(w *os.File, prompt string) (string, error) {
	fmt.Fprint(w, prompt)
	if isInteractiveTerminal() && term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(w)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func writeDotEnvFile(path string, env map[string]string) error {
	if len(env) == 0 {
		return nil
	}
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		// Respect existing .env to avoid surprising overwrites.
		return nil
	}
	lines := []string{
		"# Runtime-only secrets (never commit actual values)",
	}
	keys := []string{}
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := env[k]
		if strings.ContainsAny(v, "\n\r") {
			return fmt.Errorf("invalid value for %s (contains newline)", k)
		}
		lines = append(lines, k+"="+v)
	}
	lines = append(lines, "")
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}
	return nil
}
