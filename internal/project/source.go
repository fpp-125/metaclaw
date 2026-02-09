package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type ResolvedTemplate struct {
	Dir    string
	Commit string // git commit SHA when Kind=git; may be empty for local templates
}

func ResolveTemplate(source TemplateSource) (ResolvedTemplate, error) {
	switch source.Kind {
	case TemplateSourceKindLocal:
		if strings.TrimSpace(source.Dir) == "" {
			return ResolvedTemplate{}, errors.New("template source dir is empty")
		}
		abs, err := filepath.Abs(source.Dir)
		if err != nil {
			return ResolvedTemplate{}, fmt.Errorf("resolve template dir: %w", err)
		}
		if st, err := os.Stat(abs); err != nil {
			return ResolvedTemplate{}, fmt.Errorf("template dir not accessible: %w", err)
		} else if !st.IsDir() {
			return ResolvedTemplate{}, fmt.Errorf("template dir is not a directory: %s", abs)
		}
		return ResolvedTemplate{Dir: abs}, nil
	case TemplateSourceKindGit:
		repo := strings.TrimSpace(source.Repo)
		if repo == "" {
			return ResolvedTemplate{}, errors.New("template source repo is empty")
		}
		ref := strings.TrimSpace(source.Ref)
		if ref == "" {
			ref = "main"
		}
		sub := filepath.Clean(strings.TrimSpace(source.Path))
		if sub == "." || sub == "" || strings.HasPrefix(sub, "..") {
			return ResolvedTemplate{}, fmt.Errorf("invalid template path %q", source.Path)
		}
		if !commandExists("git") {
			return ResolvedTemplate{}, errors.New("git not found (required for git template sources)")
		}

		cacheRoot, err := defaultTemplateCacheRoot()
		if err != nil {
			return ResolvedTemplate{}, err
		}
		repoDir := filepath.Join(cacheRoot, "git", hashKey(repo))
		if err := os.MkdirAll(filepath.Dir(repoDir), 0o755); err != nil {
			return ResolvedTemplate{}, fmt.Errorf("create template cache: %w", err)
		}

		if _, err := os.Stat(repoDir); err == nil {
			// Best-effort sync. If offline, we still allow using the cached copy.
			_ = syncGitRepo(repoDir, ref)
		} else {
			if err := gitCloneShallow(cacheRoot, repo, repoDir); err != nil {
				return ResolvedTemplate{}, err
			}
			_ = syncGitRepo(repoDir, ref)
		}

		dir := filepath.Join(repoDir, sub)
		if st, err := os.Stat(dir); err != nil {
			return ResolvedTemplate{}, fmt.Errorf("template path not accessible: %w", err)
		} else if !st.IsDir() {
			return ResolvedTemplate{}, fmt.Errorf("template path is not a directory: %s", dir)
		}

		commit, _ := gitRevParse(repoDir, "HEAD")
		return ResolvedTemplate{Dir: dir, Commit: strings.TrimSpace(commit)}, nil
	default:
		return ResolvedTemplate{}, fmt.Errorf("unsupported template source kind %q", source.Kind)
	}
}

func defaultTemplateCacheRoot() (string, error) {
	// Prefer OS cache directory, fallback to temp.
	if d, err := os.UserCacheDir(); err == nil && strings.TrimSpace(d) != "" {
		return filepath.Join(d, "metaclaw", "templates"), nil
	}
	return filepath.Join(os.TempDir(), "metaclaw-templates-cache"), nil
}

func hashKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

func commandExists(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func gitCloneShallow(workDir, repoURL, dst string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, dst)
	cmd.Dir = workDir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed (repo=%s): %w", repoURL, err)
	}
	return nil
}

func syncGitRepo(repoDir, ref string) error {
	// Keep sync quiet and best-effort: offline users should still be able to run upgrades against cached templates.
	_ = runGit(repoDir, "fetch", "--prune", "--depth", "1", "origin", ref)
	_ = runGit(repoDir, "reset", "--hard", "origin/"+ref)
	_ = runGit(repoDir, "clean", "-fdx")
	return nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func gitRevParse(dir, ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func hashKeyPlatformSafe(s string) string {
	// Deprecated: kept for future if we need platform-specific cache partitioning.
	// Currently unused.
	return fmt.Sprintf("%s-%s", runtime.GOOS, hashKey(s))
}
