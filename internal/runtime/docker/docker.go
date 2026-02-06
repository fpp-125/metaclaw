package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/metaclaw/metaclaw/internal/policy"
	"github.com/metaclaw/metaclaw/internal/runtime/spec"
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) Name() spec.Target { return spec.TargetDocker }

func (a *Adapter) Available(context.Context) bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

func (a *Adapter) Run(ctx context.Context, opts spec.RunOptions) (spec.RunResult, error) {
	args := []string{"run", "--name", opts.ContainerName}
	if opts.Detach {
		args = append(args, "-d")
	}
	args = append(args, policyFlags(opts.Policy, opts.Env, opts.Workdir, opts.User)...)
	args = append(args, opts.Image)
	args = append(args, opts.Command...)
	stdout, stderr, code, err := run(ctx, "docker", args)
	if opts.Detach {
		return spec.RunResult{ContainerID: strings.TrimSpace(stdout), ExitCode: code, Stdout: stdout, Stderr: stderr}, err
	}
	return spec.RunResult{ContainerID: opts.ContainerName, ExitCode: code, Stdout: stdout, Stderr: stderr}, err
}

func (a *Adapter) Logs(ctx context.Context, containerID string, follow bool) (string, error) {
	args := []string{"logs"}
	if follow {
		args = append(args, "--follow")
	}
	args = append(args, containerID)
	stdout, stderr, _, err := run(ctx, "docker", args)
	if err != nil {
		return stdout + stderr, err
	}
	return stdout + stderr, nil
}

func (a *Adapter) Inspect(ctx context.Context, containerID string) (string, error) {
	stdout, stderr, _, err := run(ctx, "docker", []string{"inspect", containerID})
	if err != nil {
		return stdout + stderr, err
	}
	return stdout, nil
}

func (a *Adapter) ExecShell(ctx context.Context, containerID string) error {
	return interactive(ctx, "docker", []string{"exec", "-it", containerID, "sh"})
}

func (a *Adapter) Remove(ctx context.Context, containerID string) error {
	_, _, _, err := run(ctx, "docker", []string{"rm", "-f", containerID})
	return err
}

func policyFlags(p policy.Policy, env map[string]string, workdir, user string) []string {
	args := make([]string, 0)
	if p.Network.Mode == "none" {
		args = append(args, "--network=none")
	}
	for _, m := range p.Mounts {
		v := fmt.Sprintf("%s:%s", m.Source, m.Target)
		if m.ReadOnly {
			v += ":ro"
		}
		args = append(args, "-v", v)
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, env[k]))
	}
	if workdir != "" {
		args = append(args, "-w", workdir)
	}
	if user != "" {
		args = append(args, "-u", user)
	}
	return args
}

func run(ctx context.Context, bin string, args []string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exit := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
		} else {
			exit = -1
		}
	}
	return out.String(), errBuf.String(), exit, err
}

func interactive(ctx context.Context, bin string, args []string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("shell session ended with non-zero exit: %w", err)
		}
		return err
	}
	return nil
}
