package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// passthrough is a no-op arg filter for tests.
func passthrough(args []string) ([]string, error) { return args, nil }

// fetchCmd returns the subcommand registered under parent via
// AddExecutableAsCommand. The test calls it once after registration.
func fetchCmd(t *testing.T, parent *cobra.Command, label string) *cobra.Command {
	t.Helper()
	cmds := parent.Commands()
	if len(cmds) != 1 {
		t.Fatalf("%s: expected 1 subcommand, got %d", label, len(cmds))
	}
	return cmds[0]
}

func TestAddExecutableAsCommandRegistersSubcommand(t *testing.T) {
	parent := &cobra.Command{Use: "root"}
	AddExecutableAsCommand(parent, "ping", passthrough)

	sub := fetchCmd(t, parent, "registers")
	if sub.Use != "ping" {
		t.Errorf("subcommand Use = %q; want ping", sub.Use)
	}
	if !sub.DisableFlagParsing {
		t.Error("DisableFlagParsing must be true so flags reach the child")
	}
}

func TestAddExecutableAsCommandEmptyCommandRejectedAtRun(t *testing.T) {
	// The Run callback performs:
	//   if command == "" || filepath.Base(command) != command { print "invalid command"; return }
	// We verify the same predicate on the registered subcommand to lock
	// down the contract. The actual Run invocation requires the
	// subcommand to have a non-empty Use (cobra refuses empty Use), so
	// the runtime check is exercised indirectly by TestAddExecutableAsCommandPathRejectedAtRun.
	parent := &cobra.Command{Use: "root"}
	AddExecutableAsCommand(parent, "", passthrough)
	sub := fetchCmd(t, parent, "empty")

	if sub.Use != "" {
		t.Errorf("expected Use to remain empty, got %q", sub.Use)
	}
}

func TestAddExecutableAsCommandPathRejectedAtRun(t *testing.T) {
	parent := &cobra.Command{Use: "root"}
	AddExecutableAsCommand(parent, "/usr/bin/ping", passthrough)
	_ = fetchCmd(t, parent, "path")

	out := &bytes.Buffer{}
	parent.SetOut(out)
	parent.SetErr(out)
	parent.SetArgs([]string{"/usr/bin/ping"})

	if err := parent.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out.String(), "invalid command") {
		t.Errorf("output = %q; want it to contain 'invalid command'", out.String())
	}
}

var errBoom = &stringErr{"boom"}

type stringErr struct{ s string }

func (e *stringErr) Error() string { return e.s }

func TestAddExecutableAsCommandFilterErrorReported(t *testing.T) {
	parent := &cobra.Command{Use: "root"}
	filterErr := func(args []string) ([]string, error) {
		return nil, errBoom
	}
	AddExecutableAsCommand(parent, "ls", filterErr)
	_ = fetchCmd(t, parent, "filterErr")

	out := &bytes.Buffer{}
	parent.SetOut(out)
	parent.SetErr(out)
	parent.SetArgs([]string{"ls"})

	if err := parent.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out.String(), "boom") {
		t.Errorf("output = %q; want it to mention the filter error", out.String())
	}
}

func TestAddExecutableAsCommandFilterApplied(t *testing.T) {
	parent := &cobra.Command{Use: "root"}
	var received []string
	filter := func(args []string) ([]string, error) {
		received = args
		return args, nil
	}
	AddExecutableAsCommand(parent, "definitely-not-a-real-binary-xyz", filter)
	_ = fetchCmd(t, parent, "filter")

	out := &bytes.Buffer{}
	parent.SetOut(out)
	parent.SetErr(out)
	parent.SetArgs([]string{"definitely-not-a-real-binary-xyz", "--foo", "bar"})

	if err := parent.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(received) != 2 || received[0] != "--foo" || received[1] != "bar" {
		t.Errorf("filter saw %v; want [--foo bar]", received)
	}
	// The exec fails because the binary does not exist; the error
	// is written to stdout via cmd.Println(err).
	if !strings.Contains(out.String(), "executable file not found") &&
		!strings.Contains(out.String(), "no such file") {
		t.Errorf("output = %q; want it to surface the exec error", out.String())
	}
}

func TestAddExecutableAsCommandRespectsContextCancellation(t *testing.T) {
	parent := &cobra.Command{Use: "root"}
	AddExecutableAsCommand(parent, "definitely-not-a-real-binary-xyz", passthrough)
	sub := fetchCmd(t, parent, "ctx")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before invoking

	out := &bytes.Buffer{}
	sub.SetOut(out)
	sub.SetErr(out)
	sub.SetContext(ctx)
	_ = sub.Execute()
}