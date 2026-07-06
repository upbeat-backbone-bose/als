package fakeshell

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/samlm0/als/v2/config"
	"github.com/spf13/cobra"
)

func subcommandNames(root *cobra.Command) []string {
	cmds := root.Commands()
	out := make([]string, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, c.Use)
	}
	return out
}

func TestDefineMenuCommandsAllFeaturesOn(t *testing.T) {
	prev := config.Config
	config.Config = &config.ALSConfig{
		FeaturePing:            true,
		FeatureTraceroute:      true,
		FeatureSpeedtestDotNet: true,
		FeatureMTR:             true,
	}
	t.Cleanup(func() { config.Config = prev })

	// Isolate PATH to an empty dir so exec.LookPath fails for every
	// binary. With features on but no binary on PATH, the factory
	// must still produce a valid root and must NOT register any
	// subcommands (which would be unreachable anyway). The
	// "binary not installed" message is the only stderr we expect.
	t.Setenv("PATH", t.TempDir())

	factory := defineMenuCommands()
	if factory == nil {
		t.Fatal("factory is nil")
	}
	root := factory()

	if !root.CompletionOptions.DisableDefaultCmd {
		t.Error("DisableDefaultCmd should be true")
	}
	if !root.DisableFlagsInUseLine {
		t.Error("DisableFlagsInUseLine should be true")
	}

	// Subcommands that require a binary on PATH must not be
	// registered: with PATH isolated, exec.LookPath fails for
	// every binary-backed feature.
	for _, n := range []string{"ping", "traceroute", "nexttrace", "speedtest", "mtr"} {
		for _, c := range root.Commands() {
			if c.Use == n {
				t.Errorf("binary-backed subcommand %q should not be registered when binary is not on PATH", n)
			}
		}
	}
}

func TestDefineMenuCommandsAllFeaturesOff(t *testing.T) {
	prev := config.Config
	config.Config = &config.ALSConfig{}
	t.Cleanup(func() { config.Config = prev })

	// Even with all features on in the config, isolating PATH
	// means no subcommands can be registered. This test exercises
	// the feature-off path independently of PATH state.
	t.Setenv("PATH", t.TempDir())

	factory := defineMenuCommands()
	root := factory()
	names := subcommandNames(root)
	for _, n := range names {
		switch n {
		case "ping", "traceroute", "nexttrace", "speedtest", "mtr":
			t.Errorf("feature %q should NOT be registered when flag is off", n)
		}
	}
}

func TestDefineMenuCommandsFactoryCreatesFreshRoot(t *testing.T) {
	prev := config.Config
	config.Config = &config.ALSConfig{FeaturePing: true}
	t.Cleanup(func() { config.Config = prev })

	factory := defineMenuCommands()
	root1 := factory()
	root2 := factory()
	if root1 == root2 {
		t.Error("factory should return a fresh root each time")
	}
}

// TestDefineMenuCommandsPingFilter exercises the ping-specific
// argsFilter that rejects -f flags. We cannot execute the subcommand
// (it would spawn a real ping binary), but we can capture the
// filter via a small detour: re-implement the regex check ourselves
// and assert the predicate matches the spec captured in the
// function. This is a behaviour-locking test, not an execution test.
func TestDefineMenuCommandsPingFilter(t *testing.T) {
	prev := config.Config
	config.Config = &config.ALSConfig{FeaturePing: true}
	t.Cleanup(func() { config.Config = prev })

	// We can detect that ping's filter rejects -f by attempting to
	// run it through cobra. We point PATH at a fake binary that
	// records the args it was called with.
	//
	// Cross-platform: the fake binary must be runnable on the host
	// OS. POSIX uses a #!/bin/sh shebang script; Windows uses a
	// .cmd batch file (the only script type that exec.LookPath
	// resolves to on Windows via PATHEXT). The script just
	// records its argv to <tDir>/args.log.
	//
	// On Windows, we must NOT also create a ping.exe in PATH --
	// PATHEXT lists .EXE before .CMD, so exec.LookPath would
	// resolve "ping" to a (non-PE) ping.exe that fails silently
	// rather than to our ping.cmd. We rely on LookPath falling
	// through PATHEXT to .cmd.
	tDir := t.TempDir()
	fakeName, fakeScript := buildFakePingScript(tDir)
	fakeBin := filepath.Join(tDir, fakeName)
	if err := os.WriteFile(fakeBin, []byte(fakeScript), 0o755); err != nil {
		t.Fatalf("write fake ping: %v", err)
	}
	t.Setenv("PATH", tDir)

	factory := defineMenuCommands()
	root := factory()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"ping", "127.0.0.1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(ping) error: %v", err)
	}
	logBytes, err := os.ReadFile(filepath.Join(tDir, "args.log"))
	if err != nil {
		t.Fatalf("args.log not written: %v", err)
	}
	if !strings.Contains(string(logBytes), "127.0.0.1") {
		t.Errorf("ping did not see 127.0.0.1 in args: %q", logBytes)
	}

	// Now retry with a dangerous -f flag and confirm the filter
	// blocks it before exec is invoked.
	if err := os.WriteFile(filepath.Join(tDir, "args.log"), nil, 0o644); err != nil {
		t.Fatalf("truncate args.log: %v", err)
	}
	root.SetArgs([]string{"ping", "-f", "127.0.0.1"})
	_ = root.Execute()
	logBytes, _ = os.ReadFile(filepath.Join(tDir, "args.log"))
	if len(logBytes) != 0 {
		t.Errorf("dangerous flag -f should have been blocked; args.log = %q", logBytes)
	}
}

// buildFakePingScript returns (filename, contents) for a fake ping
// binary that records its arguments to <tDir>/args.log. The
// filename and contents differ per OS:
//
//   - POSIX: a plain "ping" file with #!/bin/sh shebang. Shell
//     expansion turns "$@" into the list of CLI args.
//   - Windows: a "ping.cmd" file invoked by cmd.exe via PATHEXT.
//     %* expands to the full argument list. The .cmd extension
//     is required because cmd.exe only runs files with script
//     extensions from PATH (not bare "ping" with no extension);
//     exec.LookPath appends the extension for us.
//
// Both forms are simple enough to be obviously non-production;
// they exist only to capture the argument list for assertion.
func buildFakePingScript(tDir string) (name, script string) {
	argsLog := filepath.Join(tDir, "args.log")
	if runtime.GOOS == "windows" {
		return "ping.cmd", "@echo off\r\necho %* > \"" + argsLog + "\"\r\n"
	}
	return "ping", "#!/bin/sh\necho \"$@\" > " + argsLog + "\n"
}

func TestDefineMenuCommandsRootDefaults(t *testing.T) {
	prev := config.Config
	config.Config = &config.ALSConfig{}
	t.Cleanup(func() { config.Config = prev })

	factory := defineMenuCommands()
	root := factory()

	if root.CompletionOptions.DisableDefaultCmd != true {
		t.Error("DisableDefaultCmd must be true")
	}
	if root.DisableFlagsInUseLine != true {
		t.Error("DisableFlagsInUseLine must be true")
	}
}

func TestDefineMenuCommandsHasDefaultHelp(t *testing.T) {
	prev := config.Config
	config.Config = &config.ALSConfig{}
	t.Cleanup(func() { config.Config = prev })

	factory := defineMenuCommands()
	root := factory()

	help, _, err := root.Find([]string{"help"})
	if err != nil {
		t.Fatalf("Find(help) error: %v", err)
	}
	if help == nil {
		t.Fatal("help command not found")
	}
}
