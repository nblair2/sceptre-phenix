package mmcli

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestIsTransientErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"broken pipe", errors.New("write unix: broken pipe"), true},
		{"closed conn", errors.New("read tcp: use of closed network connection"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"meshage", errors.New("meshage: timed out waiting for ACK"), true},
		{"wrapped ErrTimeout", fmt.Errorf("running cmd: %w", ErrTimeout), true},
		{"case insensitive", errors.New("BROKEN PIPE"), true},

		{"mesh send yourself", errors.New("cannot mesh send yourself"), false},
		{"vm not found", errors.New("vm not found: foo"), false},
		{"syntax", errors.New("expected one of: ..."), false},
		{"unknown", errors.New("something else entirely"), false},

		// permanent classification wins even when a transient token is present.
		{"permanent beats transient", errors.New("namespace must be active: connection reset by peer"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := IsTransientErr(tc.err); got != tc.want {
				t.Errorf("IsTransientErr(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestBackoffBounds(t *testing.T) {
	t.Parallel()

	base := 500 * time.Millisecond

	for attempt := range 12 {
		d := backoff(attempt, base)

		if d < 0 {
			t.Errorf("attempt %d: backoff %v is negative", attempt, d)
		}

		if d >= maxRetryBackoff {
			t.Errorf("attempt %d: backoff %v exceeds cap %v", attempt, d, maxRetryBackoff)
		}
	}
}

func TestBackoffZeroBaseUsesDefault(t *testing.T) {
	t.Parallel()

	// A non-positive base must not panic (rand.Int63n requires n > 0) and must
	// stay within the cap.
	for attempt := range 4 {
		if d := backoff(attempt, 0); d < 0 || d >= maxRetryBackoff {
			t.Errorf("attempt %d: backoff %v out of range with zero base", attempt, d)
		}
	}
}

func TestCommandRetriesDefault(t *testing.T) {
	t.Parallel()

	if got := (&Command{}).retries(); got != defaultRetries {
		t.Errorf("unset Retries = %d, want default %d", got, defaultRetries)
	}

	if got := (&Command{Retries: 5}).retries(); got != 5 {
		t.Errorf("Retries=5 = %d, want 5", got)
	}
}
