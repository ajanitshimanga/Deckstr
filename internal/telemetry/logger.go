package telemetry

import (
	"encoding/json"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Options configures a Logger. Zero values fall back to defaults suitable
// for the desktop app; tests override them to point at a scratch dir.
type Options struct {
	Dir         string        // directory to write logs into
	ClientID    string        // stable anonymous ID (loaded by caller)
	ServiceName string
	Version     string
	MaxSize     int64         // per-file size before rotation (bytes)
	Backups     int           // number of rotated files to retain
	FlushEvery  time.Duration // buffered flush cadence
	Now         func() time.Time // overridable clock for tests

	// PostHogAPIKey overrides the link-time-injected key. Tests use this to
	// inject a stub; production leaves it empty to fall back to the global.
	PostHogAPIKey string
	// PostHogEndpoint overrides the link-time-injected endpoint. Same role.
	PostHogEndpoint string
	// PostHogSkipEvents replaces the default skip set (which drops the
	// polling event "account.detect"). Pass an empty map to ship everything.
	PostHogSkipEvents map[string]bool
}

// Logger is the singleton event sink. All calls are safe for concurrent
// use. Failures in the I/O path are swallowed — telemetry must never fail
// the application.
type Logger struct {
	mu       sync.Mutex
	writer   *rotatingWriter
	resource map[string]interface{}
	now      func() time.Time
	shipper  *posthogShipper

	flushDone chan struct{}
	wg        sync.WaitGroup
	closed    bool
}

// New constructs a Logger and starts its flush goroutine. Close must be
// called on shutdown to flush buffered records.
func New(opts Options) (*Logger, error) {
	if opts.MaxSize == 0 {
		opts.MaxSize = defaultMaxSize
	}
	if opts.Backups == 0 {
		opts.Backups = defaultBackups
	}
	if opts.FlushEvery == 0 {
		opts.FlushEvery = time.Duration(defaultFlushEvery) * time.Second
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if opts.ClientID == "" {
		opts.ClientID = uuid.NewString() // fallback so we never emit a blank ID
	}

	w, err := newRotatingWriter(opts.Dir, logFileName, opts.MaxSize, opts.Backups)
	if err != nil {
		return nil, err
	}

	apiKey := opts.PostHogAPIKey
	if apiKey == "" {
		apiKey = posthogAPIKey
	}
	endpoint := opts.PostHogEndpoint
	if endpoint == "" {
		endpoint = posthogEndpoint
	}
	skip := opts.PostHogSkipEvents
	if skip == nil {
		skip = defaultPosthogSkipEvents
	}
	sessionID := uuid.NewString()

	l := &Logger{
		writer: w,
		now:    opts.Now,
		resource: map[string]interface{}{
			"service.name":    opts.ServiceName,
			"service.version": opts.Version,
			"os.type":         runtime.GOOS,
			"client.id":       opts.ClientID,
			// PostHog-reserved property name. Unlocks session-grouping in
			// the UI ("show me everything this user did in this session")
			// and is treated as the canonical session field by their SDKs.
			"$session_id": sessionID,
		},
		shipper:   newPosthogShipper(apiKey, endpoint, opts.ClientID, skip),
		flushDone: make(chan struct{}),
	}

	l.wg.Add(1)
	go l.flushLoop(opts.FlushEvery)
	return l, nil
}

// Log appends a record to the buffered writer. Errors are swallowed.
// attrs may be nil; keys/values should be coarse (counts, category IDs).
// DO NOT pass credentials, usernames, passwords, Riot IDs, or any other
// vault data — the whitelist-by-caller invariant is load-bearing.
func (l *Logger) Log(sev Severity, body string, attrs map[string]interface{}) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	rec := Record{
		Timestamp:      l.now(),
		SeverityText:   sev.Text(),
		SeverityNumber: int(sev),
		Body:           body,
		Attributes:     attrs,
		Resource:       l.resource,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		l.mu.Unlock()
		return
	}
	// Single write per record so the rotator sees per-record sizes and can
	// rotate between records rather than in the middle of one.
	_, _ = l.writer.Write(append(b, '\n'))
	shipper := l.shipper
	l.mu.Unlock()

	// Ship to PostHog outside the mutex - the SDK is async + buffered, so
	// this returns immediately, but we still don't want to hold the file
	// lock across SDK calls.
	if shipper != nil {
		props := make(map[string]interface{}, len(attrs)+len(l.resource)+1)
		for k, v := range attrs {
			props[k] = v
		}
		for k, v := range l.resource {
			props[k] = v
		}
		props["severity"] = sev.Text()
		shipper.capture(body, props)
	}
}

func (l *Logger) flushLoop(interval time.Duration) {
	defer l.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.flush()
		case <-l.flushDone:
			l.flush()
			return
		}
	}
}

// flush asks the OS to persist buffered pages. Writes themselves are
// already direct — this is for durability across crashes.
func (l *Logger) flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.writer != nil {
		_ = l.writer.Sync()
	}
}

// Close stops the flush loop, flushes any buffered records, and closes
// the underlying file. Safe to call more than once.
func (l *Logger) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	l.mu.Unlock()

	close(l.flushDone)
	l.wg.Wait()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.shipper.close()
	if l.writer != nil {
		return l.writer.Close()
	}
	return nil
}

// ----- Package-level singleton helpers --------------------------------
//
// The Wails App uses these from multiple goroutines. Calls before Init
// (or after Close) are no-ops, so unit tests and tools can skip setup.

var (
	defaultMu     sync.RWMutex
	defaultLogger *Logger
)

// Init builds the default logger wired to the user's config directory.
// serviceName/version are embedded in every record's resource block.
// Returns nil on success; errors are non-fatal and the caller should log
// and continue without telemetry.
func Init(serviceName, version string) error {
	dir, err := logsDir()
	if err != nil {
		return err
	}
	idPath, err := clientIDPath()
	if err != nil {
		return err
	}
	id, err := loadOrCreateClientID(idPath)
	if err != nil {
		return err
	}

	l, err := New(Options{
		Dir:         dir,
		ClientID:    id,
		ServiceName: serviceName,
		Version:     version,
	})
	if err != nil {
		return err
	}

	defaultMu.Lock()
	if defaultLogger != nil {
		_ = defaultLogger.Close()
	}
	defaultLogger = l
	defaultMu.Unlock()
	return nil
}

// Log records an event on the default logger. No-op if Init failed or
// was never called.
func Log(sev Severity, body string, attrs map[string]interface{}) {
	defaultMu.RLock()
	l := defaultLogger
	defaultMu.RUnlock()
	if l == nil {
		return
	}
	l.Log(sev, body, attrs)
}

// Close shuts down the default logger. Safe to call from shutdown paths
// even if Init was never called.
func Close() error {
	defaultMu.Lock()
	l := defaultLogger
	defaultLogger = nil
	defaultMu.Unlock()
	if l == nil {
		return nil
	}
	return l.Close()
}

// SetDefault replaces the singleton. Exported for tests that need to
// install their own logger; production code should call Init.
func SetDefault(l *Logger) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultLogger = l
}

// Helper that other packages in the Wails app import, so they don't need
// to construct the attrs map inline for the common case.
func LogInfo(body string, attrs map[string]interface{}) {
	Log(SeverityInfo, body, attrs)
}

// Helper kept for non-fatal errors (e.g., detection failures). Body
// should be a short machine-readable code; use attrs for context.
func LogError(body string, attrs map[string]interface{}) {
	Log(SeverityError, body, attrs)
}

// Read scans all log files for the current install and returns every
// record in chronological order. Intended for debugging / support flows
// where the user exports their local logs. Returns (nil, nil) if no
// log file has been written yet.
func Read() ([]Record, error) {
	dir, err := logsDir()
	if err != nil {
		return nil, err
	}
	return readDir(dir)
}

func readDir(dir string) ([]Record, error) {
	// Read oldest-first: .3, .2, .1, active
	paths := []string{}
	for i := defaultBackups; i >= 1; i-- {
		p := dir + string(os.PathSeparator) + logFileName + "." + itoa(i)
		if _, err := os.Stat(p); err == nil {
			paths = append(paths, p)
		}
	}
	active := dir + string(os.PathSeparator) + logFileName
	if _, err := os.Stat(active); err == nil {
		paths = append(paths, active)
	}

	var out []Record
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range splitLines(data) {
			if len(line) == 0 {
				continue
			}
			var r Record
			if err := json.Unmarshal(line, &r); err != nil {
				continue
			}
			out = append(out, r)
		}
	}
	return out, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
