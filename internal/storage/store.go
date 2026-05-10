// Package storage manages all runtime operational data.
// Everything persists under .splash/ in the workspace root.
//
//	.splash/
//	  knowledge/   — self-optimizing knowledge entries
//	  artifacts/   — outputs produced by workflow runs
//	  history/     — execution records
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store is the root storage system.
type Store struct {
	dir      string
	mu       sync.RWMutex
	knowledge []*KnowledgeEntry
}

// New opens or creates a store rooted at dir/.splash/.
func New(workspaceDir string) (*Store, error) {
	root := filepath.Join(workspaceDir, ".splash")
	for _, sub := range []string{"knowledge", "artifacts", "history"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return nil, fmt.Errorf("storage: init %s: %w", sub, err)
		}
	}
	s := &Store{dir: root}
	_ = s.loadKnowledge() // non-fatal
	return s, nil
}

// ── Knowledge ─────────────────────────────────────────────────────────────

// KnowledgeEntry is a single piece of operational knowledge.
// Entries are scored and reinforced over time — the store self-optimizes.
type KnowledgeEntry struct {
	ID        string            `json:"id"`
	Workflow  string            `json:"workflow"`
	Kind      string            `json:"kind"`    // "step_output" | "pattern" | "failure" | "fix"
	Content   string            `json:"content"`
	Tags      []string          `json:"tags"`
	Score     float64           `json:"score"`   // increases when useful, decreases on bad outcomes
	UsedCount int               `json:"used"`
	Meta      map[string]string `json:"meta,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// RecordKnowledge stores a new knowledge entry.
func (s *Store) RecordKnowledge(workflow, kind, content string, tags []string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	e := &KnowledgeEntry{
		ID:        id,
		Workflow:  workflow,
		Kind:      kind,
		Content:   content,
		Tags:      tags,
		Score:     1.0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.knowledge = append(s.knowledge, e)
	_ = s.persistEntry(e)
	return id
}

// QueryKnowledge returns relevant knowledge entries for a workflow context.
// Results are sorted by score — highest first.
func (s *Store) QueryKnowledge(query, workflow string, limit int) []*KnowledgeEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(query)
	var matches []*KnowledgeEntry

	for _, e := range s.knowledge {
		if workflow != "" && e.Workflow != workflow && !strings.Contains(e.Workflow, workflow) {
			continue
		}
		if strings.Contains(strings.ToLower(e.Content), q) || tagsMatch(e.Tags, q) {
			matches = append(matches, e)
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	if limit > 0 && len(matches) > limit {
		return matches[:limit]
	}
	return matches
}

// Reinforce marks a knowledge entry as useful — increases its score.
func (s *Store) Reinforce(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.knowledge {
		if e.ID == id {
			e.Score += 0.25
			e.UsedCount++
			e.UpdatedAt = time.Now()
			_ = s.persistEntry(e)
			return
		}
	}
}

// Penalize marks a knowledge entry as harmful — decreases its score.
func (s *Store) Penalize(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.knowledge {
		if e.ID == id {
			e.Score -= 0.4
			if e.Score < 0 {
				e.Score = 0
			}
			e.UpdatedAt = time.Now()
			_ = s.persistEntry(e)
			return
		}
	}
}

// KnowledgeSummary formats the top-scoring knowledge for injection
// into an agent's context during a workflow run.
func (s *Store) KnowledgeSummary(workflow string, limit int) string {
	entries := s.QueryKnowledge(workflow, workflow, limit)
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Operational knowledge from previous runs:\n")
	for _, e := range entries {
		fmt.Fprintf(&sb, "  [%s] %s\n", e.Kind, e.Content)
	}
	return sb.String()
}

// ── Artifacts ─────────────────────────────────────────────────────────────

// SaveArtifact persists an artifact to .splash/artifacts/.
func (s *Store) SaveArtifact(workflow, runID, name, kind, content string) error {
	dir := filepath.Join(s.dir, "artifacts", workflow)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("storage: artifact dir: %w", err)
	}

	type artifact struct {
		Name      string `json:"name"`
		Kind      string `json:"kind"`
		Workflow  string `json:"workflow"`
		RunID     string `json:"run_id"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
	}

	data, _ := json.MarshalIndent(artifact{
		Name:      name,
		Kind:      kind,
		Workflow:  workflow,
		RunID:     runID,
		Content:   content,
		CreatedAt: time.Now().Format(time.RFC3339),
	}, "", "  ")

	fname := filepath.Join(dir, fmt.Sprintf("%s_%s.json", runID, name))
	return os.WriteFile(fname, data, 0o644)
}

// ── History ───────────────────────────────────────────────────────────────

// RunRecord is a summary of one workflow execution.
type RunRecord struct {
	RunID    string    `json:"run_id"`
	Workflow string    `json:"workflow"`
	Success  bool      `json:"success"`
	Steps    int       `json:"steps"`
	Error    string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

// RecordRun persists an execution record.
func (s *Store) RecordRun(r RunRecord) {
	dir := filepath.Join(s.dir, "history")
	data, _ := json.MarshalIndent(r, "", "  ")
	fname := filepath.Join(dir, fmt.Sprintf("%s.json", r.RunID))
	_ = os.WriteFile(fname, data, 0o644)
}

// ── Internal ──────────────────────────────────────────────────────────────

func (s *Store) persistEntry(e *KnowledgeEntry) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, "knowledge", e.ID+".json")
	return os.WriteFile(path, data, 0o644)
}

func (s *Store) loadKnowledge() error {
	dir := filepath.Join(s.dir, "knowledge")
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var e KnowledgeEntry
		if err := json.Unmarshal(data, &e); err != nil {
			continue
		}
		s.knowledge = append(s.knowledge, &e)
	}
	return nil
}

func tagsMatch(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}
