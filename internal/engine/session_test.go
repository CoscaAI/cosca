package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
)

func TestNewSessionManager(t *testing.T) {
	t.Run("creates with custom directory", func(t *testing.T) {
		sm := NewSessionManager("/tmp/test-sessions")
		if sm.sessionsDir != "/tmp/test-sessions" {
			t.Errorf("sessionsDir = %q", sm.sessionsDir)
		}
		if sm.sessions == nil {
			t.Error("sessions map should be initialized")
		}
	})

	t.Run("creates with default directory", func(t *testing.T) {
		sm := NewSessionManagerDefault()
		if sm.sessionsDir != defaultSessionsDir {
			t.Errorf("sessionsDir = %q, want %q", sm.sessionsDir, defaultSessionsDir)
		}
	})
}

func TestCreateSession(t *testing.T) {
	t.Run("generates valid UUID format", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "cosca-general")
		if s.ID == "" {
			t.Fatal("ID should not be empty")
		}
		// UUID v4 format: 8-4-4-4-12 hex digits
		parts := strings.Split(s.ID, "-")
		if len(parts) != 5 {
			t.Errorf("UUID should have 5 parts, got %d", len(parts))
		}
		if len(parts[2]) != 4 {
			t.Errorf("UUID part 3 should be 4 chars, got %d", len(parts[2]))
		}
	})

	t.Run("sets correct fields", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "cosca-architecture")
		if s.Model != "gpt-4o" {
			t.Errorf("Model = %q", s.Model)
		}
		if s.Agent != "cosca-architecture" {
			t.Errorf("Agent = %q", s.Agent)
		}
		if s.Messages == nil {
			t.Error("Messages should be initialized")
		}
		if len(s.Messages) != 0 {
			t.Errorf("Messages should be empty, got %d", len(s.Messages))
		}
		if s.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set")
		}
		if s.UpdatedAt.IsZero() {
			t.Error("UpdatedAt should be set")
		}
		if s.Metadata == nil {
			t.Error("Metadata should be initialized")
		}
	})

	t.Run("creates unique IDs", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s1 := sm.CreateSession("gpt-4o", "general")
		s2 := sm.CreateSession("gpt-4o", "general")
		if s1.ID == s2.ID {
			t.Error("session IDs should be unique")
		}
	})
}

func TestGetSession(t *testing.T) {
	t.Run("returns session from memory", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		created := sm.CreateSession("gpt-4o", "test")
		got, err := sm.GetSession(created.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("ID = %q, want %q", got.ID, created.ID)
		}
	})

	t.Run("returns error for unknown session", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		_, err := sm.GetSession("nonexistent-id")
		if err == nil {
			t.Error("expected error for unknown session")
		}
	})
}

func TestAppendMessage(t *testing.T) {
	t.Run("adds message to session", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "test")
		msg := chat.Message{Role: chat.RoleUser, Content: "Hello!"}

		err := sm.AppendMessage(s.ID, msg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check the message was appended
		s, _ = sm.GetSession(s.ID)
		if len(s.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(s.Messages))
		}
		if s.Messages[0].Content != "Hello!" {
			t.Errorf("message content = %q", s.Messages[0].Content)
		}
	})

	t.Run("multiple messages are appended in order", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "test")
		_ = sm.AppendMessage(s.ID, chat.Message{Role: chat.RoleUser, Content: "First"})
		_ = sm.AppendMessage(s.ID, chat.Message{Role: chat.RoleAssistant, Content: "Second"})
		_ = sm.AppendMessage(s.ID, chat.Message{Role: chat.RoleUser, Content: "Third"})

		s, _ = sm.GetSession(s.ID)
		if len(s.Messages) != 3 {
			t.Fatalf("expected 3 messages, got %d", len(s.Messages))
		}
		if s.Messages[0].Content != "First" || s.Messages[2].Content != "Third" {
			t.Errorf("messages out of order: %v", s.Messages)
		}
	})

	t.Run("returns error for unknown session", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		err := sm.AppendMessage("nonexistent", chat.Message{})
		if err == nil {
			t.Error("expected error for unknown session")
		}
	})

	t.Run("updates the UpdatedAt timestamp", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "test")
		original := s.UpdatedAt

		time.Sleep(time.Millisecond)
		_ = sm.AppendMessage(s.ID, chat.Message{Role: chat.RoleUser, Content: "Hi"})

		s, _ = sm.GetSession(s.ID)
		if !s.UpdatedAt.After(original) {
			t.Error("UpdatedAt should be updated after append")
		}
	})
}

func TestSaveSession(t *testing.T) {
	t.Run("persists to disk", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "cosca-test")
		_ = sm.AppendMessage(s.ID, chat.Message{Role: chat.RoleUser, Content: "Hello"})
		_ = sm.AppendMessage(s.ID, chat.Message{Role: chat.RoleAssistant, Content: "Hi there"})

		err := sm.SaveSession(s.ID)
		if err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}

		// Check file exists
		path := filepath.Join(dir, s.ID+".jsonl")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatal("session file was not created")
		}

		// Verify file contents
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 lines (meta + 2 messages), got %d", len(lines))
		}

		// First line should be meta
		var meta jsonlLine
		if err := json.Unmarshal([]byte(lines[0]), &meta); err != nil {
			t.Fatalf("parse meta line: %v", err)
		}
		if meta.Type != "meta" {
			t.Errorf("first line type = %q, want meta", meta.Type)
		}
		if meta.ID != s.ID {
			t.Errorf("meta ID = %q", meta.ID)
		}
	})

	t.Run("creates directory if needed", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "sessions")
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "test")
		err := sm.SaveSession(s.ID)
		if err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Error("sessions directory should have been created")
		}
	})

	t.Run("returns error for unknown session", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		err := sm.SaveSession("nonexistent")
		if err == nil {
			t.Error("expected error for unknown session")
		}
	})

	t.Run("saves usage line when tokens > 0", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "test")
		s.TokenUsage = chat.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
		_ = sm.AppendMessage(s.ID, chat.Message{Role: chat.RoleUser, Content: "Hi"})

		err := sm.SaveSession(s.ID)
		if err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}

		// Read file and check for usage line
		path := filepath.Join(dir, s.ID+".jsonl")
		data, _ := os.ReadFile(path)
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		lastLine := lines[len(lines)-1]

		var last jsonlLine
		json.Unmarshal([]byte(lastLine), &last)
		if last.Type != "usage" {
			t.Errorf("last line type = %q, want usage", last.Type)
		}
		if last.TotalTokens != 30 {
			t.Errorf("TotalTokens = %d, want 30", last.TotalTokens)
		}
	})

	t.Run("does not save usage line when tokens is 0", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "test")
		_ = sm.AppendMessage(s.ID, chat.Message{Role: chat.RoleUser, Content: "Hi"})

		err := sm.SaveSession(s.ID)
		if err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}

		path := filepath.Join(dir, s.ID+".jsonl")
		data, _ := os.ReadFile(path)
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines {
			var parsed jsonlLine
			json.Unmarshal([]byte(line), &parsed)
			if parsed.Type == "usage" {
				t.Error("should not have usage line when tokens = 0")
			}
		}
	})
}

func TestLoadSession(t *testing.T) {
	t.Run("reads session from disk", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		created := sm.CreateSession("gpt-4o", "cosca-test")
		_ = sm.AppendMessage(created.ID, chat.Message{Role: chat.RoleUser, Content: "Hello"})
		_ = sm.SaveSession(created.ID)

		// Create a new manager to read back
		sm2 := NewSessionManager(dir)
		loaded, err := sm2.LoadSession(created.ID)
		if err != nil {
			t.Fatalf("LoadSession failed: %v", err)
		}
		if loaded.ID != created.ID {
			t.Errorf("ID = %q", loaded.ID)
		}
		if len(loaded.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(loaded.Messages))
		}
		if loaded.Messages[0].Content != "Hello" {
			t.Errorf("message content = %q", loaded.Messages[0].Content)
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		_, err := sm.LoadSession("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})

	t.Run("handles corrupt file gracefully", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "test")
		_ = sm.SaveSession(s.ID)

		// Corrupt the file
		path := filepath.Join(dir, s.ID+".jsonl")
		_ = os.WriteFile(path, []byte("{invalid json}\n"), 0644)

		_, err := sm.LoadSession(s.ID)
		if err == nil {
			t.Error("expected error for corrupt file")
		}
	})
}

func TestDeleteSession(t *testing.T) {
	t.Run("removes session file from disk", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "test")
		_ = sm.SaveSession(s.ID)

		err := sm.DeleteSession(s.ID)
		if err != nil {
			t.Fatalf("DeleteSession failed: %v", err)
		}

		path := filepath.Join(dir, s.ID+".jsonl")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("file should have been deleted")
		}

		// Should also be removed from memory
		_, err = sm.GetSession(s.ID)
		if err == nil {
			t.Error("session should be removed from memory")
		}
	})

	t.Run("succeeds when file does not exist", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "test")
		// Don't save, just delete from memory
		err := sm.DeleteSession(s.ID)
		if err != nil {
			t.Fatalf("DeleteSession should not fail for non-existent file: %v", err)
		}
	})
}

func TestListSessions(t *testing.T) {
	t.Run("returns sessions sorted by updated time", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		// Create sessions and save them
		s1 := sm.CreateSession("gpt-4o", "test")
		_ = sm.SaveSession(s1.ID)
		time.Sleep(10 * time.Millisecond)

		s2 := sm.CreateSession("gpt-4o", "test")
		_ = sm.SaveSession(s2.ID)
		time.Sleep(10 * time.Millisecond)

		s3 := sm.CreateSession("gpt-4o", "test")
		_ = sm.SaveSession(s3.ID)

		// Create a new manager to list from disk
		sm2 := NewSessionManager(dir)
		sessions, err := sm2.ListSessions()
		if err != nil {
			t.Fatalf("ListSessions failed: %v", err)
		}
		if len(sessions) != 3 {
			t.Fatalf("expected 3 sessions, got %d", len(sessions))
		}
		// Most recent first
		if sessions[0].ID != s3.ID {
			t.Errorf("first should be most recent: got %q, want %q", sessions[0].ID, s3.ID)
		}
	})

	t.Run("returns empty list for non-existent directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nonexistent")
		sm := NewSessionManager(dir)

		sessions, err := sm.ListSessions()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 0 {
			t.Errorf("expected 0 sessions, got %d", len(sessions))
		}
	})

	t.Run("skips non-JSONL files", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		// Create a valid session
		s := sm.CreateSession("gpt-4o", "test")
		_ = sm.SaveSession(s.ID)

		// Create some non-jsonl files
		_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644)
		_ = os.WriteFile(filepath.Join(dir, "data.csv"), []byte("a,b,c"), 0644)

		sessions, err := sm.ListSessions()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 1 {
			t.Errorf("expected 1 session (skipped non-jsonl files), got %d", len(sessions))
		}
	})

	t.Run("skips corrupt files gracefully", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		// Create a valid session
		s := sm.CreateSession("gpt-4o", "test")
		_ = sm.SaveSession(s.ID)

		// Create a corrupt jsonl file
		_ = os.WriteFile(filepath.Join(dir, "corrupt.jsonl"), []byte("{not json}\n"), 0644)

		sessions, err := sm.ListSessions()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 1 {
			t.Errorf("expected 1 valid session, got %d", len(sessions))
		}
	})
}

func TestResumeLatest(t *testing.T) {
	t.Run("returns most recent session", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s1 := sm.CreateSession("gpt-4o", "test")
		_ = sm.SaveSession(s1.ID)
		time.Sleep(10 * time.Millisecond)

		s2 := sm.CreateSession("gpt-4o", "test")
		_ = sm.SaveSession(s2.ID)

		sm2 := NewSessionManager(dir)
		latest, err := sm2.ResumeLatest("gpt-4o")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if latest == nil {
			t.Fatal("expected a session")
		}
		if latest.ID != s2.ID {
			t.Errorf("got %q, want %q (most recent)", latest.ID, s2.ID)
		}
	})

	t.Run("returns nil when no sessions exist", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		latest, err := sm.ResumeLatest("gpt-4o")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if latest != nil {
			t.Error("expected nil when no sessions exist")
		}
	})
}

func TestSessionManagerHandlesMissingDirectory(t *testing.T) {
	t.Run("creates session without directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "no-such-dir")
		sm := NewSessionManager(dir)

		// Creating a session should work (in-memory only)
		s := sm.CreateSession("gpt-4o", "test")
		if s == nil {
			t.Fatal("session should have been created")
		}

		// AppendMessage should work (in-memory only)
		err := sm.AppendMessage(s.ID, chat.Message{Role: chat.RoleUser, Content: "Hello"})
		if err != nil {
			t.Fatalf("AppendMessage failed: %v", err)
		}

		// SaveSession should create the directory
		err = sm.SaveSession(s.ID)
		if err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}

		// Directory should now exist
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Error("directory should have been created by SaveSession")
		}
	})
}

func TestSessionManagerCorruptFiles(t *testing.T) {
	t.Run("handles corrupt JSONL lines gracefully", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		// Create a session with some messages
		s := sm.CreateSession("gpt-4o", "test")
		_ = sm.AppendMessage(s.ID, chat.Message{Role: chat.RoleUser, Content: "Hi"})

		// Manually create a corrupt JSONL file
		path := filepath.Join(dir, s.ID+".jsonl")
		content := `{"type":"meta","id":"` + s.ID + `","model":"gpt-4o"}
{"type":"message",invalid
`
		_ = os.WriteFile(path, []byte(content), 0644)

		// Should fail to load
		_, err := sm.LoadSession(s.ID)
		if err == nil {
			t.Error("expected error for corrupt JSONL")
		}
	})

	t.Run("handles empty file", func(t *testing.T) {
		dir := t.TempDir()
		sm := NewSessionManager(dir)

		s := sm.CreateSession("gpt-4o", "test")
		path := filepath.Join(dir, s.ID+".jsonl")
		_ = os.WriteFile(path, []byte{}, 0644)

		loaded, err := sm.LoadSession(s.ID)
		if err != nil {
			t.Fatalf("LoadSession failed: %v", err)
		}
		if loaded == nil {
			t.Fatal("expected empty session")
		}
		if len(loaded.Messages) != 0 {
			t.Errorf("expected 0 messages, got %d", len(loaded.Messages))
		}
	})
}

func TestSessionManagerConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	sm := NewSessionManager(dir)

	s := sm.CreateSession("gpt-4o", "test")

	var wg sync.WaitGroup
	const goroutines = 20

	// Concurrent appends
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := chat.Message{
				Role:    chat.RoleUser,
				Content: "Message " + string(rune('0'+idx)),
			}
			_ = sm.AppendMessage(s.ID, msg)
		}(i)
	}
	wg.Wait()

	// Check all messages were added
	s, _ = sm.GetSession(s.ID)
	if len(s.Messages) != goroutines {
		t.Errorf("expected %d messages, got %d", goroutines, len(s.Messages))
	}

	// Concurrent read/write
	var wg2 sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			_, _ = sm.GetSession(s.ID)
			_ = sm.AppendMessage(s.ID, chat.Message{Role: chat.RoleUser, Content: "more"})
			_ = sm.SaveSession(s.ID)
		}()
	}
	wg2.Wait()
}
