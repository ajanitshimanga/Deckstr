package telemetry

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestLogger(t *testing.T, maxSize int64, backups int) (*Logger, string) {
	t.Helper()
	dir := t.TempDir()
	l, err := New(Options{
		Dir:         dir,
		ClientID:    "test-client",
		ServiceName: "test",
		Version:     "0.0.0-test",
		MaxSize:     maxSize,
		Backups:     backups,
		FlushEvery:  50 * time.Millisecond,
		Now:         func() time.Time { return time.Unix(0, 0).UTC() },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l, dir
}

func TestLogger_WritesOTelRecord(t *testing.T) {
	l, dir := newTestLogger(t, 1<<20, 3)
	l.Log(SeverityInfo, "app.start", map[string]interface{}{"latency_ms": 42})
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, logFileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var r Record
	line := strings.TrimRight(string(data), "\n")
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if r.Body != "app.start" {
		t.Errorf("body = %q, want app.start", r.Body)
	}
	if r.SeverityText != "INFO" || r.SeverityNumber != int(SeverityInfo) {
		t.Errorf("severity = (%s, %d), want (INFO, %d)", r.SeverityText, r.SeverityNumber, SeverityInfo)
	}
	if r.Attributes["latency_ms"].(float64) != 42 {
		t.Errorf("latency_ms = %v, want 42", r.Attributes["latency_ms"])
	}
	for _, k := range []string{"service.name", "service.version", "os.type", "client.id", "$session_id"} {
		if _, ok := r.Resource[k]; !ok {
			t.Errorf("resource missing %q", k)
		}
	}
	if r.Resource["client.id"] != "test-client" {
		t.Errorf("client.id = %v, want test-client", r.Resource["client.id"])
	}
}

func TestLogger_RotatesWhenMaxSizeExceeded(t *testing.T) {
	// Keep maxSize small enough that every second record triggers rotation.
	l, dir := newTestLogger(t, 200, 2)

	for i := 0; i < 10; i++ {
		l.Log(SeverityInfo, "noise", map[string]interface{}{"i": i, "pad": strings.Repeat("x", 80)})
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name()] = true
	}
	if !names[logFileName] {
		t.Errorf("expected active %s in %v", logFileName, names)
	}
	// At least one rotated file should have been produced.
	if !names[logFileName+".1"] {
		t.Errorf("expected at least one rotated backup in %v", names)
	}
	// Never retain more than backups+1 files total.
	if len(names) > 3 {
		t.Errorf("expected ≤3 log files, got %d: %v", len(names), names)
	}
}

func TestLogger_ConcurrentCallersDoNotCorruptLines(t *testing.T) {
	l, dir := newTestLogger(t, 1<<20, 3)

	done := make(chan struct{})
	for g := 0; g < 4; g++ {
		go func(id int) {
			for i := 0; i < 50; i++ {
				l.Log(SeverityInfo, "concurrent", map[string]interface{}{"goroutine": id, "i": i})
			}
			done <- struct{}{}
		}(g)
	}
	for i := 0; i < 4; i++ {
		<-done
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f, err := os.Open(filepath.Join(dir, logFileName))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	count := 0
	for scanner.Scan() {
		var r Record
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			t.Fatalf("line %d not valid JSON: %v (%q)", count, err, scanner.Text())
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if count != 4*50 {
		t.Errorf("wrote %d records, want %d", count, 4*50)
	}
}

func TestLogger_LogAfterCloseIsNoOp(t *testing.T) {
	l, dir := newTestLogger(t, 1<<20, 3)
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	l.Log(SeverityInfo, "after-close", nil) // must not panic

	data, _ := os.ReadFile(filepath.Join(dir, logFileName))
	if strings.Contains(string(data), "after-close") {
		t.Errorf("record written after Close()")
	}
}

func TestLoadOrCreateClientID_ReusesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client.id")

	id1, err := loadOrCreateClientID(path)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if id1 == "" {
		t.Fatal("empty id")
	}

	id2, err := loadOrCreateClientID(path)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if id1 != id2 {
		t.Errorf("id regenerated: %q -> %q", id1, id2)
	}
}

func TestLoadOrCreateClientID_RegeneratesOnCorruption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client.id")
	if err := os.WriteFile(path, []byte("not-a-uuid"), 0600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	id, err := loadOrCreateClientID(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if id == "not-a-uuid" {
		t.Error("did not regenerate on corrupted file")
	}
}
