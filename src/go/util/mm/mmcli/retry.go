package mmcli

import (
	"errors"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/activeshadow/libminimega/miniclient"
)

const (
	defaultRetries   = 3
	defaultRetryBase = 500 * time.Millisecond
	maxRetryBackoff  = 8 * time.Second
)

// transientSubstrings are matched (case-insensitively) against an error's text.
// These represent conditions that commonly clear on a retry, especially when a
// command is fanned out to remote nodes over minimega's mesh (meshage).
var transientSubstrings = []string{ //nolint:gochecknoglobals // lookup table
	"broken pipe",
	"no such file or directory", // socket transiently gone (e.g. minimega restart)
	"use of closed network connection",
	"connection refused",
	"connection reset",
	"server disconnected", // miniclient EOF
	"i/o timeout",
	"meshage", // generic meshage transport errors
	"timeout",
}

// permanentSubstrings are matched (case-insensitively) and ALWAYS win over a
// transient match. These are genuine logic/usage errors that will never clear
// on retry. Notably "cannot mesh send yourself" contains no transient token but
// must never be retried.
var permanentSubstrings = []string{ //nolint:gochecknoglobals // lookup table
	"cannot mesh send yourself",
	"vm not found",
	"vm not running",
	"namespace must be active",
	"invalid command",
	"expected", // minicli syntax errors ("expected ...")
	"no such handler",
}

// isPermanentErr reports whether err is a permanent error that must not be
// retried.
func isPermanentErr(err error) bool {
	if err == nil {
		return false
	}

	s := strings.ToLower(err.Error())

	for _, p := range permanentSubstrings {
		if strings.Contains(s, p) {
			return true
		}
	}

	return false
}

// IsTransientErr reports whether err is worth retrying. Permanent matches
// short-circuit to false so they are never retried. It is exported so callers
// outside this package (e.g. the mm package's polling loops) can make the same
// transient-vs-permanent distinction.
func IsTransientErr(err error) bool {
	if err == nil || isPermanentErr(err) {
		return false
	}

	if errors.Is(err, ErrTimeout) {
		return true
	}

	s := strings.ToLower(err.Error())

	for _, t := range transientSubstrings {
		if strings.Contains(s, t) {
			return true
		}
	}

	return false
}

// backoff returns an exponential backoff duration with full jitter, capped at
// maxRetryBackoff. attempt is zero-based.
func backoff(attempt int, base time.Duration) time.Duration {
	if base <= 0 {
		base = defaultRetryBase
	}

	d := base << attempt // base * 2^attempt
	if d > maxRetryBackoff || d <= 0 {
		d = maxRetryBackoff
	}

	return time.Duration(rand.Int64N(int64(d)))
}

// retries returns the effective retry count for a command, applying the package
// default when unset.
func (c *Command) retries() int {
	if c.Retries <= 0 {
		return defaultRetries
	}

	return c.Retries
}

// runOnce runs the command exactly once, fully draining the response channel
// into a buffered, already-closed channel and reporting the first error
// encountered. Fully draining is required before a retry can be attempted: a
// partially-read response channel keeps the underlying miniclient connection
// lock held, which would deadlock the next attempt.
func runOnce(c *Command) (chan *miniclient.Response, error) {
	var (
		buf    []*miniclient.Response
		errStr string
	)

	for resp := range Run(c) {
		buf = append(buf, resp)

		for _, r := range resp.Resp {
			if r.Error != "" && errStr == "" {
				errStr = r.Error
			}
		}
	}

	out := make(chan *miniclient.Response, len(buf))
	for _, r := range buf {
		out <- r
	}

	close(out)

	if errStr != "" {
		return out, errors.New(errStr)
	}

	return out, nil
}

// RunWithRetry runs the command, retrying on transient errors using the
// command's Retries/RetryBase (falling back to package defaults). The returned
// channel is fully buffered and already closed, so existing consumers
// (ErrorResponse, SingleResponse, SingleDataResponse) work unchanged.
//
// Because the entire response is materialized in memory before being returned,
// RunWithRetry should only be used for small responses (tabular/status output).
// Large or streaming responses (screenshots, raw C2 output) should use Run.
//
// Only use RunWithRetry for idempotent commands: a non-idempotent command (e.g.
// `vm launch`) could be executed more than once.
func RunWithRetry(c *Command) chan *miniclient.Response {
	attempts := c.retries()

	var last chan *miniclient.Response

	for attempt := 0; attempt <= attempts; attempt++ {
		out, err := runOnce(c)
		if err == nil || !IsTransientErr(err) {
			return out
		}

		last = out

		if attempt < attempts {
			time.Sleep(backoff(attempt, c.RetryBase))
		}
	}

	// Retries exhausted; return the last (error-bearing) response set so the
	// caller surfaces the underlying error.
	return last
}

// RunTabularWithRetry mirrors RunTabularErr but retries transient failures using
// the command's Retries/RetryBase. Only use it for idempotent read commands.
func RunTabularWithRetry(c *Command) ([]map[string]string, error) {
	attempts := c.retries()

	var (
		rows []map[string]string
		err  error
	)

	for attempt := 0; attempt <= attempts; attempt++ {
		rows, err = RunTabularErr(c)
		if err == nil || !IsTransientErr(err) {
			return rows, err
		}

		if attempt < attempts {
			time.Sleep(backoff(attempt, c.RetryBase))
		}
	}

	return rows, err
}
