package task

import (
	"sync"
	"testing"
	"time"
)

func TestManager_Create(t *testing.T) {
	m := NewManager()
	req := TaskRequest{Type: TaskTypeThumbnail}
	task := m.Create(req)

	if task.ID == "" {
		t.Error("task ID should not be empty")
	}
	if task.Status != TaskStatusQueued {
		t.Errorf("status = %s, want queued", task.Status)
	}
	if task.Type != TaskTypeThumbnail {
		t.Errorf("type = %s, want thumbnail", task.Type)
	}
	if task.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if task.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestManager_Create_UniqueIDs(t *testing.T) {
	m := NewManager()
	ids := make(map[string]bool)
	for range 100 {
		task := m.Create(TaskRequest{Type: TaskTypeMetadata})
		if ids[task.ID] {
			t.Fatalf("duplicate task ID: %s", task.ID)
		}
		ids[task.ID] = true
	}
}

func TestManager_Get(t *testing.T) {
	m := NewManager()
	created := m.Create(TaskRequest{Type: TaskTypeTranscode})

	got, err := m.Get(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %s, want %s", got.ID, created.ID)
	}
}

func TestManager_Get_NotFound(t *testing.T) {
	m := NewManager()
	_, err := m.Get("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestManager_SetProcessing(t *testing.T) {
	m := NewManager()
	task := m.Create(TaskRequest{Type: TaskTypeThumbnail})

	m.SetProcessing(task.ID)

	got, _ := m.Get(task.ID)
	if got.Status != TaskStatusProcessing {
		t.Errorf("status = %s, want processing", got.Status)
	}
	if !got.UpdatedAt.After(got.CreatedAt) || got.UpdatedAt.Equal(got.CreatedAt) {
		t.Log("UpdatedAt should be >= CreatedAt")
	}
}

func TestManager_SetProcessing_NonExistent(t *testing.T) {
	m := NewManager()
	m.SetProcessing("nonexistent")
}

func TestManager_SetCompleted(t *testing.T) {
	m := NewManager()
	task := m.Create(TaskRequest{Type: TaskTypeMetadata})

	result := &TaskResult{MIMEType: "video/mp4"}
	m.SetCompleted(task.ID, result)

	got, _ := m.Get(task.ID)
	if got.Status != TaskStatusCompleted {
		t.Errorf("status = %s, want completed", got.Status)
	}
	if got.Result == nil {
		t.Fatal("result should not be nil")
	}
	if got.Result.MIMEType != "video/mp4" {
		t.Errorf("result MIME = %s, want video/mp4", got.Result.MIMEType)
	}
}

func TestManager_SetFailed(t *testing.T) {
	m := NewManager()
	task := m.Create(TaskRequest{Type: TaskTypeTranscode})

	m.SetFailed(task.ID, "ffmpeg crashed")

	got, _ := m.Get(task.ID)
	if got.Status != TaskStatusFailed {
		t.Errorf("status = %s, want failed", got.Status)
	}
	if got.Error != "ffmpeg crashed" {
		t.Errorf("error = %q, want %q", got.Error, "ffmpeg crashed")
	}
}

func TestManager_Delete(t *testing.T) {
	m := NewManager()
	task := m.Create(TaskRequest{Type: TaskTypeThumbnail})

	if !m.Delete(task.ID) {
		t.Error("Delete should return true for existing task")
	}

	_, err := m.Get(task.ID)
	if err == nil {
		t.Error("Get should fail after Delete")
	}
}

func TestManager_Delete_NonExistent(t *testing.T) {
	m := NewManager()
	if m.Delete("nope") {
		t.Error("Delete should return false for nonexistent task")
	}
}

func TestManager_CleanupOlderThan(t *testing.T) {
	m := NewManager()

	old := m.Create(TaskRequest{Type: TaskTypeThumbnail})
	m.SetCompleted(old.ID, &TaskResult{})
	m.mu.Lock()
	m.tasks[old.ID].UpdatedAt = time.Now().Add(-2 * time.Hour)
	m.mu.Unlock()

	recent := m.Create(TaskRequest{Type: TaskTypeMetadata})
	m.SetCompleted(recent.ID, &TaskResult{})

	queued := m.Create(TaskRequest{Type: TaskTypeTranscode})
	m.mu.Lock()
	m.tasks[queued.ID].UpdatedAt = time.Now().Add(-2 * time.Hour)
	m.mu.Unlock()

	removed := m.CleanupOlderThan(1 * time.Hour)

	if removed != 1 {
		t.Errorf("removed = %d, want 1 (only old completed)", removed)
	}

	if _, err := m.Get(old.ID); err == nil {
		t.Error("old completed task should be cleaned up")
	}
	if _, err := m.Get(recent.ID); err != nil {
		t.Error("recent completed task should still exist")
	}
	if _, err := m.Get(queued.ID); err != nil {
		t.Error("queued task should not be cleaned up regardless of age")
	}
}

func TestManager_CleanupOlderThan_FailedTasks(t *testing.T) {
	m := NewManager()
	task := m.Create(TaskRequest{Type: TaskTypeThumbnail})
	m.SetFailed(task.ID, "error")
	m.mu.Lock()
	m.tasks[task.ID].UpdatedAt = time.Now().Add(-2 * time.Hour)
	m.mu.Unlock()

	removed := m.CleanupOlderThan(1 * time.Hour)
	if removed != 1 {
		t.Errorf("removed = %d, want 1 (old failed task)", removed)
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup

	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task := m.Create(TaskRequest{Type: TaskTypeThumbnail})
			m.SetProcessing(task.ID)
			if i%2 == 0 {
				m.SetCompleted(task.ID, &TaskResult{})
			} else {
				m.SetFailed(task.ID, "test error")
			}
			m.Get(task.ID)
			m.Delete(task.ID)
		}()
	}
	wg.Wait()
}

func TestManager_FullLifecycle(t *testing.T) {
	m := NewManager()

	task := m.Create(TaskRequest{Type: TaskTypeTranscode})
	if task.Status != TaskStatusQueued {
		t.Fatalf("initial status = %s, want queued", task.Status)
	}

	m.SetProcessing(task.ID)
	got, _ := m.Get(task.ID)
	if got.Status != TaskStatusProcessing {
		t.Fatalf("after SetProcessing = %s, want processing", got.Status)
	}

	result := &TaskResult{
		MIMEType: "video/mp4",
		Output:   &TranscodeResult{Key: "output.mp4", Size: 1024},
	}
	m.SetCompleted(task.ID, result)
	got, _ = m.Get(task.ID)
	if got.Status != TaskStatusCompleted {
		t.Fatalf("after SetCompleted = %s, want completed", got.Status)
	}
	if got.Result.Output.Key != "output.mp4" {
		t.Errorf("output key = %s, want output.mp4", got.Result.Output.Key)
	}

	m.Delete(task.ID)
	_, err := m.Get(task.ID)
	if err == nil {
		t.Error("should not find deleted task")
	}
}
