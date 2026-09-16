package watcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ────────────────────────────────────────────────────────────────

type Config struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}

func jsonParse(data []byte) (Config, error) {
	var c Config
	return c, json.Unmarshal(data, &c)
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, content, 0o644))
}

func tempFile(t *testing.T, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, content)
	return path
}

func receiveWithTimeout[T any](ch <-chan T, timeout time.Duration) (T, bool) {
	select {
	case v, ok := <-ch:
		return v, ok
	case <-time.After(timeout):
		var zero T
		return zero, false
	}
}

// ── New() — initialisation ─────────────────────────────────────────────────

func TestNew_ReturnsNonNilCancelAndChannel(t *testing.T) {
	path := tempFile(t, []byte(`{"port":8080}`))

	cancel, ch, err := New(path, jsonParse, 1)

	require.NoError(t, err)
	assert.NotNil(t, cancel)
	assert.NotNil(t, ch)
	cancel()
}

func TestNew_InvalidPath_ReturnsError(t *testing.T) {
	// filepath.Abs never fails on valid strings, but a non-existent
	// *directory* causes Add to fail.
	cancel, ch, err := New("/nonexistent/dir/config.json", jsonParse, 1)

	assert.Error(t, err)
	assert.Nil(t, cancel)
	assert.Nil(t, ch)
}

func TestNew_BufferedChannel_RespectsCapacity(t *testing.T) {
	path := tempFile(t, []byte(`{"port":9090}`))

	cancel, ch, err := New(path, jsonParse, 5)
	require.NoError(t, err)
	defer cancel()

	assert.Equal(t, 5, cap(ch))
}

func TestNew_UnbufferedChannel(t *testing.T) {
	path := tempFile(t, []byte(`{"port":9090}`))

	cancel, ch, err := New(path, jsonParse, 0)
	require.NoError(t, err)
	defer cancel()

	assert.Equal(t, 0, cap(ch))
}

// ── cancel() behaviour ─────────────────────────────────────────────────────

func TestCancel_ClosesChannel(t *testing.T) {
	path := tempFile(t, []byte(`{"port":1234}`))

	cancel, ch, err := New(path, jsonParse, 1)
	require.NoError(t, err)

	cancel()

	// After cancel the goroutine closes ch; wait briefly.
	_, open := receiveWithTimeout(ch, time.Second)
	assert.False(t, open, "channel should be closed after cancel")
}

func TestCancel_IsIdempotent(t *testing.T) {
	path := tempFile(t, []byte(`{"port":1234}`))

	cancel, ch, err := New(path, jsonParse, 1)
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		cancel()
		cancel() // second call must not panic
	})
	<-ch // drain / wait for close
}

// ── file-change detection ──────────────────────────────────────────────────

func TestWatcher_DetectsInPlaceWrite(t *testing.T) {
	path := tempFile(t, []byte(`{"port":3000}`))

	cancel, ch, err := New(path, jsonParse, 1)
	require.NoError(t, err)
	defer cancel()

	// Overwrite the file in-place.
	writeFile(t, path, []byte(`{"port":4000,"host":"localhost"}`))

	got, ok := receiveWithTimeout(ch, 2*time.Second)
	require.True(t, ok, "expected a value after file write")
	assert.Equal(t, 4000, got.Port)
	assert.Equal(t, "localhost", got.Host)
}

func TestWatcher_DetectsAtomicReplacement(t *testing.T) {
	path := tempFile(t, []byte(`{"port":5000}`))

	cancel, ch, err := New(path, jsonParse, 1)
	require.NoError(t, err)
	defer cancel()

	// Simulate atomic mv: write to a temp file then rename into place.
	dir := filepath.Dir(path)
	tmp := filepath.Join(dir, "config.json.tmp")
	writeFile(t, tmp, []byte(`{"port":6000}`))
	require.NoError(t, os.Rename(tmp, path))

	got, ok := receiveWithTimeout(ch, 2*time.Second)
	require.True(t, ok, "expected a value after atomic rename")
	assert.Equal(t, 6000, got.Port)
}

func TestWatcher_IgnoresUnrelatedFiles(t *testing.T) {
	path := tempFile(t, []byte(`{"port":7000}`))

	cancel, ch, err := New(path, jsonParse, 1)
	require.NoError(t, err)
	defer cancel()

	// Write to a different file in the same directory.
	other := filepath.Join(filepath.Dir(path), "other.json")
	writeFile(t, other, []byte(`{"port":9999}`))

	_, ok := receiveWithTimeout(ch, 500*time.Millisecond)
	assert.False(t, ok, "should not receive event for unrelated file")
}

// ── parse error handling ───────────────────────────────────────────────────

func TestWatcher_ParseError_DoesNotSendToChannel(t *testing.T) {
	path := tempFile(t, []byte(`{"port":8000}`))

	cancel, ch, err := New(path, jsonParse, 1)
	require.NoError(t, err)
	defer cancel()

	// Write invalid JSON — parse will fail, nothing should be sent.
	writeFile(t, path, []byte(`not valid json`))

	_, ok := receiveWithTimeout(ch, 600*time.Millisecond)
	assert.False(t, ok, "parse error should suppress channel send")
}

func TestWatcher_RecoverAfterParseError(t *testing.T) {
	path := tempFile(t, []byte(`{"port":8080}`))

	cancel, ch, err := New(path, jsonParse, 2)
	require.NoError(t, err)
	defer cancel()

	// First write: bad JSON — no event.
	writeFile(t, path, []byte(`bad`))
	time.Sleep(300 * time.Millisecond)

	// Second write: valid JSON — should produce an event.
	writeFile(t, path, []byte(`{"port":9090}`))

	got, ok := receiveWithTimeout(ch, 2*time.Second)
	require.True(t, ok, "expected recovery after valid write")
	assert.Equal(t, 9090, got.Port)
}

// ── custom parse function ──────────────────────────────────────────────────

func TestNew_CustomParseFunc(t *testing.T) {
	path := tempFile(t, []byte("hello"))

	parseLine := func(data []byte) (string, error) {
		return string(data), nil
	}

	cancel, ch, err := New(path, parseLine, 1)
	require.NoError(t, err)
	defer cancel()

	writeFile(t, path, []byte("world"))

	got, ok := receiveWithTimeout(ch, 2*time.Second)
	require.True(t, ok)
	assert.Equal(t, "world", got)
}
