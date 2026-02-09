package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type TemplateSourceKind string

const (
	TemplateSourceKindLocal TemplateSourceKind = "local"
	TemplateSourceKindGit   TemplateSourceKind = "git"
)

// TemplateSource is persisted in the project lock so `metaclaw project upgrade`
// can refresh managed files deterministically.
type TemplateSource struct {
	Kind TemplateSourceKind `json:"kind"`

	// Local directory source.
	Dir string `json:"dir,omitempty"`

	// Git source.
	Repo string `json:"repo,omitempty"`
	Ref  string `json:"ref,omitempty"`  // e.g. main
	Path string `json:"path,omitempty"` // subdir within repo
}

// ProjectLock is written into the host data dir.
//
// This file is not a "bot business" artifact; it is a small piece of control-plane metadata
// that lets MetaClaw upgrade managed template files without overwriting user-owned data.
type ProjectLock struct {
	SchemaVersion int            `json:"schemaVersion"`
	Template      TemplateSource `json:"template"`

	TemplateID     string `json:"templateId"`
	TemplateCommit string `json:"templateCommit,omitempty"`
	InstalledAtUTC string `json:"installedAtUtc"`

	// ManagedFiles stores the sha256 of managed files as they existed after the last init/upgrade.
	// Key is slash-separated project-relative path.
	ManagedFiles map[string]string `json:"managedFiles,omitempty"`
}

const LockFilename = "project.lock.json"

func DefaultHostDataDir(projectDir string) string {
	return filepath.Join(projectDir, ".metaclaw")
}

func LockPath(hostDataDir string) string {
	return filepath.Join(hostDataDir, LockFilename)
}

func LoadLock(hostDataDir string) (ProjectLock, error) {
	path := LockPath(hostDataDir)
	b, err := os.ReadFile(path)
	if err != nil {
		return ProjectLock{}, err
	}
	var l ProjectLock
	if err := json.Unmarshal(b, &l); err != nil {
		return ProjectLock{}, fmt.Errorf("parse lock: %w", err)
	}
	if l.SchemaVersion == 0 {
		l.SchemaVersion = 1
	}
	if l.SchemaVersion != 1 {
		return ProjectLock{}, fmt.Errorf("unsupported lock schemaVersion %d", l.SchemaVersion)
	}
	if l.TemplateID == "" {
		return ProjectLock{}, fmt.Errorf("lock templateId is required (%s)", path)
	}
	if l.ManagedFiles == nil {
		l.ManagedFiles = map[string]string{}
	}
	return l, nil
}

func WriteLock(hostDataDir string, lock ProjectLock) error {
	if lock.SchemaVersion == 0 {
		lock.SchemaVersion = 1
	}
	if lock.InstalledAtUTC == "" {
		lock.InstalledAtUTC = time.Now().UTC().Format(time.RFC3339)
	}
	if lock.ManagedFiles == nil {
		lock.ManagedFiles = map[string]string{}
	}
	if err := os.MkdirAll(hostDataDir, 0o755); err != nil {
		return fmt.Errorf("create host data dir: %w", err)
	}
	b, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lock: %w", err)
	}
	path := LockPath(hostDataDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write lock temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("write lock: %w", err)
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum), nil
}
