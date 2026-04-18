package subcmd

import (
	"io"
	"os"

	"github.com/spf13/cobra"
)

// writerTTY reports whether w looks like an interactive terminal. Kept as a
// package variable so tests can override it without introducing non-stdlib deps.
var writerTTY = func(w io.Writer) bool {
	type fdWriter interface {
		Fd() uintptr
	}

	fw, ok := w.(fdWriter)
	if !ok {
		return false
	}
	f := os.NewFile(fw.Fd(), "")
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// AddQuietFlag registers --quiet on cmd. When enabled, informational notes are
// suppressed while errors continue to flow through Cobra normally.
func AddQuietFlag(cmd *cobra.Command, dest *bool) {
	cmd.Flags().BoolVar(dest, "quiet", false, "Suppress informational stderr notes")
}

// infoWriter returns stderr when informational output should be shown, or
// io.Discard when the user requested quiet mode or stderr is not a TTY.
func infoWriter(stderr io.Writer, quiet bool) io.Writer {
	if quiet || !writerTTY(stderr) {
		return io.Discard
	}
	return stderr
}
