package project

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type UpgradeOptions struct {
	ProjectDir  string
	HostDataDir string
	Template    TemplateSource
	Force       bool
	DryRun      bool
}

type UpgradeResult struct {
	TemplateID     string
	TemplateCommit string
	Updated        []string
	Added          []string
	Skipped        []string
	Conflicts      []string
}

func Upgrade(opts UpgradeOptions) (UpgradeResult, error) {
	if strings.TrimSpace(opts.ProjectDir) == "" {
		return UpgradeResult{}, errors.New("project dir is empty")
	}
	projectDir, err := filepath.Abs(opts.ProjectDir)
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("resolve project dir: %w", err)
	}
	hostDataDir := strings.TrimSpace(opts.HostDataDir)
	if hostDataDir == "" {
		hostDataDir = DefaultHostDataDir(projectDir)
	} else {
		hostDataDir, err = filepath.Abs(hostDataDir)
		if err != nil {
			return UpgradeResult{}, fmt.Errorf("resolve host data dir: %w", err)
		}
	}

	// Load lock (if present) to detect local modifications of managed files.
	lock, lockErr := LoadLock(hostDataDir)

	resolved, err := ResolveTemplate(opts.Template)
	if err != nil {
		return UpgradeResult{}, err
	}
	manifest, err := LoadManifest(resolved.Dir)
	if err != nil {
		return UpgradeResult{}, err
	}

	managed, err := expandManagedFiles(resolved.Dir, manifest.Managed, manifest.User)
	if err != nil {
		return UpgradeResult{}, err
	}
	if len(managed) == 0 {
		return UpgradeResult{}, fmt.Errorf("manifest managed patterns matched 0 files")
	}

	backupRoot := filepath.Join(hostDataDir, "upgrade-backups", time.Now().UTC().Format("20060102T150405Z"))
	out := UpgradeResult{
		TemplateID:     manifest.ID,
		TemplateCommit: strings.TrimSpace(resolved.Commit),
		Updated:        []string{},
		Added:          []string{},
		Skipped:        []string{},
		Conflicts:      []string{},
	}

	// Sort for stable output.
	sort.Strings(managed)

	managedHashes := map[string]string{}
	if lockErr == nil {
		managedHashes = lock.ManagedFiles
	}

	for _, rel := range managed {
		src := filepath.Join(resolved.Dir, filepath.FromSlash(rel))
		dst := filepath.Join(projectDir, filepath.FromSlash(rel))

		// Skip if destination is explicitly user-owned (belt-and-suspenders).
		if matchAny(rel, manifest.User) {
			out.Skipped = append(out.Skipped, rel)
			continue
		}

		// Detect local modification (best-effort).
		if lockErr == nil {
			if prev, ok := managedHashes[rel]; ok && prev != "" {
				if cur, err := sha256File(dst); err == nil {
					if cur != prev && !opts.Force {
						out.Conflicts = append(out.Conflicts, rel)
						continue
					}
					if cur != prev && opts.Force {
						if !opts.DryRun {
							if err := backupFile(dst, filepath.Join(backupRoot, filepath.FromSlash(rel))); err != nil {
								return out, err
							}
						}
					}
				}
			}
		}

		existed, err := fileExists(dst)
		if err != nil {
			return out, err
		}
		if opts.DryRun {
			if existed {
				out.Updated = append(out.Updated, rel)
			} else {
				out.Added = append(out.Added, rel)
			}
			continue
		}

		if err := copyFilePreserveMode(src, dst); err != nil {
			return out, fmt.Errorf("copy %s: %w", rel, err)
		}
		if existed {
			out.Updated = append(out.Updated, rel)
		} else {
			out.Added = append(out.Added, rel)
		}

		if sum, err := sha256File(dst); err == nil {
			managedHashes[rel] = sum
		}
	}

	// Write/refresh lock after a successful upgrade. If there were conflicts, do not advance the lock
	// unless we were forced; otherwise repeated upgrades would overwrite conflict detection.
	if opts.DryRun {
		return out, nil
	}
	if len(out.Conflicts) > 0 && !opts.Force {
		return out, fmt.Errorf("upgrade has conflicts (%d files); re-run with --force to overwrite or resolve locally", len(out.Conflicts))
	}

	newLock := ProjectLock{
		SchemaVersion:  1,
		Template:       opts.Template,
		TemplateID:     manifest.ID,
		TemplateCommit: strings.TrimSpace(resolved.Commit),
		InstalledAtUTC: time.Now().UTC().Format(time.RFC3339),
		ManagedFiles:   managedHashes,
	}
	// If we had an existing lock and it loaded, preserve any fields not regenerated.
	if lockErr == nil {
		if newLock.Template.Kind == "" {
			newLock.Template = lock.Template
		}
	}

	if err := WriteLock(hostDataDir, newLock); err != nil {
		return out, err
	}
	return out, nil
}

func fileExists(path string) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return st.Mode().IsRegular(), nil
}

func backupFile(srcPath, dstPath string) error {
	ok, err := fileExists(srcPath)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	if err := copyFilePreserveMode(srcPath, dstPath); err != nil {
		return fmt.Errorf("backup file: %w", err)
	}
	return nil
}

func copyFilePreserveMode(src, dst string) error {
	st, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlinks are not supported in templates (%s)", src)
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", src)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, st.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	// Preserve executable bit, etc.
	_ = os.Chmod(dst, st.Mode().Perm())
	return nil
}

func expandManagedFiles(templateDir string, managedPatterns, userPatterns []string) ([]string, error) {
	userPatterns = normalizePatterns(userPatterns)
	managedPatterns = normalizePatterns(managedPatterns)

	// Walk once; match in-memory.
	files := make([]string, 0, 256)
	err := filepath.WalkDir(templateDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" {
				return filepath.SkipDir
			}
			if name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".pyc") {
			return nil
		}
		rel, err := filepath.Rel(templateDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ManifestFilename {
			// Manifest itself is not managed unless explicitly listed.
			// Keeping it out avoids overwriting user-owned manifests if projects choose to copy it.
			return nil
		}
		if matchAny(rel, userPatterns) {
			return nil
		}
		if matchAny(rel, managedPatterns) || matchByDirPresence(templateDir, rel, managedPatterns) {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Deduplicate
	seen := map[string]struct{}{}
	out := make([]string, 0, len(files))
	for _, f := range files {
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out, nil
}

func normalizePatterns(in []string) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = strings.TrimPrefix(p, "./")
		p = strings.TrimSuffix(p, "/")
		out = append(out, filepath.ToSlash(p))
	}
	return out
}

func matchAny(rel string, patterns []string) bool {
	rel = filepath.ToSlash(rel)
	for _, pat := range patterns {
		pat = filepath.ToSlash(pat)
		if strings.HasSuffix(pat, "/**") {
			prefix := strings.TrimSuffix(pat, "/**")
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
			continue
		}
		if strings.ContainsAny(pat, "*?[") {
			ok, _ := path.Match(pat, rel)
			if ok {
				return true
			}
			continue
		}
		if rel == pat {
			return true
		}
	}
	return false
}

func matchByDirPresence(templateDir, rel string, patterns []string) bool {
	// If a pattern is a directory name without globbing (e.g. "bot" or "image"),
	// treat it as "dir/**" for convenience.
	for _, pat := range patterns {
		pat = strings.TrimSpace(filepath.ToSlash(pat))
		if pat == "" || strings.ContainsAny(pat, "*?[") || strings.HasSuffix(pat, "/**") {
			continue
		}
		info, err := os.Stat(filepath.Join(templateDir, filepath.FromSlash(pat)))
		if err != nil || !info.IsDir() {
			continue
		}
		if rel == pat || strings.HasPrefix(rel, pat+"/") {
			return true
		}
	}
	return false
}
