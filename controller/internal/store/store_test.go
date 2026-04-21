package store

import (
	"testing"
	"time"
)

func TestSummaryStartsEmpty(t *testing.T) {
	s, err := New(t.TempDir() + "/controller.sqlite")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()

	summary := s.Summary()
	if got := summary["totalNodes"]; got != 0 {
		t.Fatalf("expected 0 nodes, got %v", got)
	}
	if got := summary["onlineNodes"]; got != 0 {
		t.Fatalf("expected 0 online nodes, got %v", got)
	}
}

func TestAddTaskPrependsNewestTask(t *testing.T) {
	s, err := New(t.TempDir() + "/controller.sqlite")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	first := Task{ID: "task-1", Command: "uptime", StartedAt: time.Now().UTC().Add(-time.Minute)}
	second := Task{ID: "task-2", Command: "date", StartedAt: time.Now().UTC()}

	s.AddTask(first)
	s.AddTask(second)

	tasks := s.Tasks()
	if len(tasks) < 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "task-2" {
		t.Fatalf("expected newest task first, got %s", tasks[0].ID)
	}
}
