package plog

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A freshly written entry must be returned for a recent query window. Run
// with TZ set to a non-UTC zone (as on deployed hosts) this catches the
// local-vs-UTC timestamp parse mismatch that made the web UI Logs tab
// always return zero entries.
func TestGetLogsReturnsRecentEntry(t *testing.T) {
	AddFileHandler(filepath.Join(t.TempDir(), "phenix.log"), GetDefaultFileHandlerOpts())

	Info(TypeSystem, "recent entry for GetLogs test")

	logs, err := GetLogs(time.Now().Add(-10*time.Minute), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}

	for _, l := range logs {
		if strings.Contains(l.Message, "recent entry for GetLogs test") {
			return
		}
	}

	t.Fatalf("entry not found in recent window (got %d entries)", len(logs))
}
