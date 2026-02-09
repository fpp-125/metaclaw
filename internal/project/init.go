package project

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type InitOptions struct {
	ProjectDir  string
	HostDataDir string
	Template    TemplateSource
	Force       bool
}

type InitResult struct {
	TemplateID     string
	TemplateCommit string
	CreatedFiles   int
}

func Init(opts InitOptions) (InitResult, error) {
	if strings.TrimSpace(opts.ProjectDir) == "" {
		return InitResult{}, errors.New("project dir is empty")
	}
	projectDir, err := filepath.Abs(opts.ProjectDir)
	if err != nil {
		return InitResult{}, fmt.Errorf("resolve project dir: %w", err)
	}
	hostDataDir := strings.TrimSpace(opts.HostDataDir)
	if hostDataDir == "" {
		hostDataDir = DefaultHostDataDir(projectDir)
	} else {
		hostDataDir, err = filepath.Abs(hostDataDir)
		if err != nil {
			return InitResult{}, fmt.Errorf("resolve host data dir: %w", err)
		}
	}

	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return InitResult{}, fmt.Errorf("create project dir: %w", err)
	}
	if !opts.Force {
		entries, err := os.ReadDir(projectDir)
		if err != nil {
			return InitResult{}, fmt.Errorf("read project dir: %w", err)
		}
		allowedTop := map[string]struct{}{"": {}}
		var unexpected []string
		for _, e := range entries {
			name := e.Name()
			if name == ".DS_Store" {
				continue
			}
			if _, ok := allowedTop[name]; ok {
				continue
			}
			unexpected = append(unexpected, name)
		}
		if len(unexpected) > 0 {
			sort.Strings(unexpected)
			return InitResult{}, fmt.Errorf("project dir is not empty: %s (unexpected: %s; use --force to continue)", projectDir, strings.Join(unexpected, ", "))
		}
	}

	resolved, err := ResolveTemplate(opts.Template)
	if err != nil {
		return InitResult{}, err
	}
	manifest, err := LoadManifest(resolved.Dir)
	if err != nil {
		return InitResult{}, err
	}

	// Copy the entire template directory into the project (excluding template manifest and .git).
	created, err := copyTemplateDir(resolved.Dir, projectDir)
	if err != nil {
		return InitResult{}, err
	}

	managed, err := expandManagedFiles(resolved.Dir, manifest.Managed, manifest.User)
	if err != nil {
		return InitResult{}, err
	}
	managedHashes := map[string]string{}
	for _, rel := range managed {
		dst := filepath.Join(projectDir, filepath.FromSlash(rel))
		if sum, err := sha256File(dst); err == nil {
			managedHashes[rel] = sum
		}
	}

	lock := ProjectLock{
		SchemaVersion:  1,
		Template:       opts.Template,
		TemplateID:     manifest.ID,
		TemplateCommit: strings.TrimSpace(resolved.Commit),
		InstalledAtUTC: time.Now().UTC().Format(time.RFC3339),
		ManagedFiles:   managedHashes,
	}
	if err := WriteLock(hostDataDir, lock); err != nil {
		return InitResult{}, err
	}
	return InitResult{
		TemplateID:     manifest.ID,
		TemplateCommit: strings.TrimSpace(resolved.Commit),
		CreatedFiles:   created,
	}, nil
}

func copyTemplateDir(srcDir, dstDir string) (int, error) {
	created := 0
	err := filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dstDir, filepath.FromSlash(rel)), 0o755)
		}
		if rel == ManifestFilename {
			// The project lock stores this metadata; keeping the manifest out avoids confusion.
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not supported in templates (%s)", p)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := copyFilePreserveMode(p, filepath.Join(dstDir, filepath.FromSlash(rel))); err != nil {
			return err
		}
		created++
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return created, err
	}
	return created, nil
}
