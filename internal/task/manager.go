package task

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager tracks task lifecycle and provides thread-safe access.
type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewManager creates a new task manager.
func NewManager() *Manager {
	return &Manager{
		tasks: make(map[string]*Task),
	}
}

// Create registers a new task and returns it.
func (m *Manager) Create(req TaskRequest) *Task {
	now := time.Now()
	t := &Task{
		ID:        uuid.New().String(),
		Type:      req.Type,
		Status:    TaskStatusQueued,
		Request:   req,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.mu.Lock()
	m.tasks[t.ID] = t
	m.mu.Unlock()

	return t
}

// Get returns a task by ID.
func (m *Manager) Get(id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %s not found", id)
	}
	return t, nil
}

// SetProcessing marks a task as processing.
func (m *Manager) SetProcessing(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t, ok := m.tasks[id]; ok {
		t.Status = TaskStatusProcessing
		t.UpdatedAt = time.Now()
	}
}

// SetCompleted marks a task as completed with results.
func (m *Manager) SetCompleted(id string, result *TaskResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t, ok := m.tasks[id]; ok {
		t.Status = TaskStatusCompleted
		t.Result = result
		t.UpdatedAt = time.Now()
	}
}

// SetFailed marks a task as failed with an error message.
func (m *Manager) SetFailed(id string, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t, ok := m.tasks[id]; ok {
		t.Status = TaskStatusFailed
		t.Error = errMsg
		t.UpdatedAt = time.Now()
	}
}

// Delete removes a task. Returns true if the task was found and removed.
func (m *Manager) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[id]; ok {
		delete(m.tasks, id)
		return true
	}
	return false
}

// CleanupOlderThan removes completed/failed tasks older than the given duration.
func (m *Manager) CleanupOlderThan(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for id, t := range m.tasks {
		if (t.Status == TaskStatusCompleted || t.Status == TaskStatusFailed) && t.UpdatedAt.Before(cutoff) {
			delete(m.tasks, id)
			removed++
		}
	}
	return removed
}
