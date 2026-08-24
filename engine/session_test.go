package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestSessionManager(t *testing.T) *SessionManager {
	t.Helper()
	s, err := NewSessionManagerInDir(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionManagerInDir() error = %v", err)
	}
	return s
}

func readSessionEntries(t *testing.T, path string) []SessionEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	var entries []SessionEntry
	for lineNum, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry SessionEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("line %d is not valid JSON: %v", lineNum+1, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestAddUserEntry_WritesToDisk(t *testing.T) {
	s := newTestSessionManager(t)
	if err := s.AddUserEntry("hello, world"); err != nil {
		t.Fatalf("AddUserEntry() error = %v", err)
	}
	entries := readSessionEntries(t, s.SessionPath())
	if got, want := len(entries), 1; got != want {
		t.Fatalf("persisted entry count = %d, want %d", got, want)
	}
	if got, want := entries[0].Type, EntryTypeUser; got != want {
		t.Errorf("persisted type = %q, want %q", got, want)
	}
	if got, want := entries[0].Content, "hello, world"; got != want {
		t.Errorf("persisted content = %q, want %q", got, want)
	}
	if entries[0].ID == "" {
		t.Error("persisted entry ID is empty")
	}
	if entries[0].Timestamp == 0 {
		t.Error("persisted entry timestamp is zero")
	}
}

func TestSessionManager_RoundTrip(t *testing.T) {
	original := newTestSessionManager(t)
	if err := original.AddUserEntry("hello"); err != nil {
		t.Fatal(err)
	}
	if err := original.AddAssistantEntry("I can help", []SessionToolCall{{ID: "call-1", Name: "bash", Arguments: `{"command":"pwd"}`}}); err != nil {
		t.Fatal(err)
	}
	if err := original.AddToolResultEntry("call-1", "bash", "/tmp/project"); err != nil {
		t.Fatal(err)
	}
	if err := original.AddAssistantEntry("done", nil); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSessionManager(original.SessionPath())
	if err != nil {
		t.Fatalf("LoadSessionManager() error = %v", err)
	}
	got, want := loaded.Entries(), original.Entries()
	if len(got) != len(want) {
		t.Fatalf("loaded entry count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Type != want[i].Type || got[i].Content != want[i].Content || got[i].ToolCallID != want[i].ToolCallID {
			t.Errorf("entry %d = %#v, want %#v", i, got[i], want[i])
		}
	}
	messages := loaded.BuildMessages()
	if got, want := len(messages), 4; got != want {
		t.Fatalf("rebuilt message count = %d, want %d", got, want)
	}
	encoded, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("Marshal(rebuilt messages) error = %v", err)
	}
	for _, want := range []string{"hello", "I can help", "call-1", "bash", "/tmp/project", "done"} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("rebuilt messages do not contain %q: %s", want, encoded)
		}
	}
}

func TestLoadSessionManager_SkipsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	content := strings.Join([]string{`{"type":"user","id":"u1","timestamp":1,"content":"before"}`, `not valid json`, `{"type":"assistant","id":"a1","timestamp":2,"content":"after"}`, ""}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSessionManager(path)
	if err != nil {
		t.Fatalf("LoadSessionManager() error = %v", err)
	}
	if got, want := s.Len(), 2; got != want {
		t.Fatalf("loaded entry count = %d, want %d", got, want)
	}
	if got, want := s.Entries()[0].Content, "before"; got != want {
		t.Errorf("first content = %q, want %q", got, want)
	}
	if got, want := s.Entries()[1].Content, "after"; got != want {
		t.Errorf("second content = %q, want %q", got, want)
	}
}

func TestSessionManager_AppendDoesNotOverwrite(t *testing.T) {
	s := newTestSessionManager(t)
	for _, content := range []string{"first", "second"} {
		if err := s.AddUserEntry(content); err != nil {
			t.Fatal(err)
		}
	}
	entries := readSessionEntries(t, s.SessionPath())
	if got, want := len(entries), 2; got != want {
		t.Fatalf("persisted entry count = %d, want %d", got, want)
	}
	if entries[0].Content != "first" || entries[1].Content != "second" {
		t.Errorf("persisted contents = %q, %q", entries[0].Content, entries[1].Content)
	}
}

func TestSessionManager_EntriesReturnsCopy(t *testing.T) {
	s := newTestSessionManager(t)
	if err := s.AddUserEntry("original"); err != nil {
		t.Fatal(err)
	}
	entries := s.Entries()
	entries[0].Content = "mutated"
	if got, want := s.Entries()[0].Content, "original"; got != want {
		t.Errorf("internal content = %q after mutating copy, want %q", got, want)
	}
}

func TestSessionManager_TruncateAfter(t *testing.T) {
	s := newTestSessionManager(t)
	for _, content := range []string{"one", "two", "three"} {
		if err := s.AddUserEntry(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.TruncateAfter(2); err != nil {
		t.Fatalf("TruncateAfter() error = %v", err)
	}
	if got, want := s.Len(), 2; got != want {
		t.Fatalf("in-memory entry count = %d, want %d", got, want)
	}
	entries := readSessionEntries(t, s.SessionPath())
	if got, want := len(entries), 2; got != want {
		t.Fatalf("persisted entry count = %d, want %d", got, want)
	}
	if entries[1].Content != "two" {
		t.Errorf("second persisted content = %q, want %q", entries[1].Content, "two")
	}
}
