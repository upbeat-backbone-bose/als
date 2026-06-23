package fakeshell

import (
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

	// Each binary-backed subcommand is only registered when both
	// the feature flag is on and the binary is on PATH. We just verify
	// the factory runs without panic and the root is correctly set up.
	// The exact subcommands depend on which binaries are installed.
	_ = subcommandNames(root)
}

func TestDefineMenuCommandsAllFeaturesOff(t *testing.T) {
	prev := config.Config
	config.Config = &config.ALSConfig{}
	t.Cleanup(func() { config.Config = prev })

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