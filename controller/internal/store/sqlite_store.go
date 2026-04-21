package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.ensureBootstrapAudit(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.cleanupLegacyData(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			last_seen_at TEXT NOT NULL,
			payload_json TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			status TEXT NOT NULL,
			payload_json TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audits (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			payload_json TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("migrate sqlite: %w", err)
		}
	}
	return nil
}

func (s *Store) ensureBootstrapAudit() error {
	count := 0
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM audits`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	s.AddAudit(AuditRecord{
		ID:        "audit-1",
		Actor:     "system",
		Action:    "bootstrap",
		Target:    "cluster",
		CreatedAt: time.Now().UTC(),
	})
	return nil
}

func (s *Store) cleanupLegacyData() error {
	statements := []string{
		`DELETE FROM nodes WHERE id LIKE 'node-demo-%' OR payload_json LIKE '%"address":"http://%'`,
		`DELETE FROM tasks WHERE payload_json LIKE '%node-demo-%'`,
		`DELETE FROM audits WHERE payload_json LIKE '%node-demo-%'`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("cleanup legacy data: %w", err)
		}
	}
	return nil
}

func (s *Store) UpsertNode(node NodeDetail) {
	node.Online = isOnline(node.LastSeenAt)
	payload, _ := json.Marshal(node)
	_, _ = s.db.Exec(
		`INSERT INTO nodes (id, name, last_seen_at, payload_json)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name,
		   last_seen_at = excluded.last_seen_at,
		   payload_json = excluded.payload_json`,
		node.ID,
		node.Name,
		node.LastSeenAt.UTC().Format(time.RFC3339Nano),
		string(payload),
	)
}

func (s *Store) ListNodes() []NodeSummary {
	rows, err := s.db.Query(`SELECT payload_json FROM nodes`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]NodeSummary, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			continue
		}
		var node NodeDetail
		if json.Unmarshal([]byte(payload), &node) != nil {
			continue
		}
		node.Online = isOnline(node.LastSeenAt)
		items = append(items, node.NodeSummary)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (s *Store) GetNode(id string) (NodeDetail, bool) {
	var payload string
	err := s.db.QueryRow(`SELECT payload_json FROM nodes WHERE id = ?`, id).Scan(&payload)
	if err != nil {
		return NodeDetail{}, false
	}
	var node NodeDetail
	if json.Unmarshal([]byte(payload), &node) != nil {
		return NodeDetail{}, false
	}
	node.Online = isOnline(node.LastSeenAt)
	return node, true
}

func (s *Store) AddTask(task Task) {
	payload, _ := json.Marshal(task)
	finished := ""
	if task.FinishedAt != nil {
		finished = task.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	_, _ = s.db.Exec(
		`INSERT INTO tasks (id, started_at, finished_at, status, payload_json)
		 VALUES (?, ?, ?, ?, ?)`,
		task.ID,
		task.StartedAt.UTC().Format(time.RFC3339Nano),
		finished,
		task.Status,
		string(payload),
	)
}

func (s *Store) Tasks() []Task {
	rows, err := s.db.Query(`SELECT payload_json FROM tasks ORDER BY started_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]Task, 0)
	for rows.Next() {
		var payload string
		if rows.Scan(&payload) != nil {
			continue
		}
		var task Task
		if json.Unmarshal([]byte(payload), &task) != nil {
			continue
		}
		items = append(items, task)
	}
	return items
}

func (s *Store) AddAudit(record AuditRecord) {
	payload, _ := json.Marshal(record)
	_, _ = s.db.Exec(
		`INSERT OR REPLACE INTO audits (id, created_at, payload_json)
		 VALUES (?, ?, ?)`,
		record.ID,
		record.CreatedAt.UTC().Format(time.RFC3339Nano),
		string(payload),
	)
}

func (s *Store) Audits() []AuditRecord {
	rows, err := s.db.Query(`SELECT payload_json FROM audits ORDER BY created_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]AuditRecord, 0)
	for rows.Next() {
		var payload string
		if rows.Scan(&payload) != nil {
			continue
		}
		var audit AuditRecord
		if json.Unmarshal([]byte(payload), &audit) != nil {
			continue
		}
		items = append(items, audit)
	}
	return items
}

func (s *Store) AddRecentCommand(nodeID string, result TaskResult) {
	node, ok := s.GetNode(nodeID)
	if !ok {
		return
	}
	node.RecentCommands = append([]TaskResult{result}, node.RecentCommands...)
	if len(node.RecentCommands) > 10 {
		node.RecentCommands = node.RecentCommands[:10]
	}
	s.UpsertNode(node)
}

func (s *Store) Alerts() []map[string]any {
	nodes := s.ListNodes()
	alerts := make([]map[string]any, 0)
	for _, summary := range nodes {
		node, ok := s.GetNode(summary.ID)
		if !ok {
			continue
		}
		for _, alert := range node.RecentAlerts {
			alerts = append(alerts, map[string]any{
				"nodeId":   node.ID,
				"nodeName": node.Name,
				"severity": alert.Severity,
				"message":  alert.Message,
				"at":       alert.At,
			})
		}
	}
	sort.Slice(alerts, func(i, j int) bool {
		return fmt.Sprint(alerts[i]["at"]) > fmt.Sprint(alerts[j]["at"])
	})
	return alerts
}
