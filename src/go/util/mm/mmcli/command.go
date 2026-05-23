// Taken (almost) as-is from minimega/miniweb.

package mmcli

import (
	"fmt"
	"strings"
	"time"
)

// Command represents a command and options to send to minimega.
type Command struct {
	Command   string
	Columns   []string
	Filters   []string
	Namespace string
	Timeout   time.Duration

	// Retries is the maximum number of times to retry the command when it fails
	// with a transient error (see IsTransientErr). It only takes effect when the
	// command is run through one of the *WithRetry helpers. A value <= 0 falls
	// back to defaultRetries. Only set this on idempotent commands: retrying a
	// non-idempotent command (e.g. `vm launch`) risks executing it more than once.
	Retries int

	// RetryBase is the base delay used for exponential backoff between retries. A
	// value <= 0 falls back to defaultRetryBase.
	RetryBase time.Duration
}

// NewCommand returns a pointer to a new, initialized command.
func NewCommand() *Command {
	return new(Command)
}

// NewNamespacedCommand returns a pointer to a new command, initialized with the
// given minimega namespace name.
func NewNamespacedCommand(ns string) *Command {
	return &Command{Namespace: ns} //nolint:exhaustruct // partial initialization
}

// String builds the actual command string to send to minimega using the command
// fields.
func (c *Command) String() string {
	cmd := c.Command

	// Apply filters first so we don't need to worry about the columns not
	// including the filtered fields.
	for _, f := range c.Filters {
		cmd = fmt.Sprintf(".filter %v %v", f, cmd)
	}

	if len(c.Columns) > 0 {
		columns := make([]string, len(c.Columns))

		// Quote all the columns in case there are spaces.
		for i := range c.Columns {
			columns[i] = fmt.Sprintf("%q", c.Columns[i])
		}

		cmd = fmt.Sprintf(".columns %v %v", strings.Join(columns, ","), cmd)
	}

	// If there's a namespace, use it.
	if c.Namespace != "" {
		cmd = fmt.Sprintf("namespace %q %v", c.Namespace, cmd)
	}

	// Don't record command in history.
	cmd = fmt.Sprintf(".record false %v", cmd)

	return cmd
}
