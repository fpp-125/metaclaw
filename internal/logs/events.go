package logs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Event struct {
	Timestamp   string `json:"timestamp"`
	RunID       string `json:"runId"`
	Phase       string `json:"phase"`
	Runtime     string `json:"runtime,omitempty"`
	ContainerID string `json:"containerId,omitempty"`
	Message     string `json:"message"`
	Error       string `json:"error,omitempty"`
}

func AppendEvent(stateDir string, runID string, e Event) error {
	e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	e.RunID = runID
	path := filepath.Join(stateDir, "runs", runID, "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func ReadEvents(stateDir string, runID string) ([]string, error) {
	path := filepath.Join(stateDir, "runs", runID, "events.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	s := bufio.NewScanner(f)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("scan events: %w", err)
	}
	return lines, nil
}
