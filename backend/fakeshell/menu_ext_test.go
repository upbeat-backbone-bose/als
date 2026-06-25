package fakeshell

import (
	"testing"

	"github.com/reeflective/console"
	"github.com/samlm0/als/v2/config"
)

func TestSetupPromptSetsAllOptions(t *testing.T) {
	app := console.New("test-app")
	m := app.ActiveMenu()
	setupPrompt(m)

	p := m.Prompt()
	if p == nil {
		t.Fatal("prompt is nil after setupPrompt")
	}
	if p.Primary == nil {
		t.Error("Primary prompt function not set")
	}
	if p.Secondary == nil {
		t.Error("Secondary prompt function not set")
	}
	if p.Transient == nil {
		t.Error("Transient prompt function not set")
	}
}

func TestSetupPromptPrimaryReturnsNonEmpty(t *testing.T) {
	app := console.New("test-app")
	m := app.ActiveMenu()
	setupPrompt(m)

	primary := m.Prompt().Primary()
	if primary == "" {
		t.Error("Primary prompt must not be empty")
	}
}

func TestSetupPromptSecondaryReturnsNonEmpty(t *testing.T) {
	app := console.New("test-app")
	m := app.ActiveMenu()
	setupPrompt(m)

	secondary := m.Prompt().Secondary()
	if secondary == "" {
		t.Error("Secondary prompt must not be empty")
	}
}

func TestSetupPromptTransientReturnsNonEmpty(t *testing.T) {
	app := console.New("test-app")
	m := app.ActiveMenu()
	setupPrompt(m)

	transient := m.Prompt().Transient()
	if transient == "" {
		t.Error("Transient prompt must not be empty")
	}
}

func TestDefineMenuCommandsFactoryIsConsistent(t *testing.T) {
	prev := config.Config
	config.Config = &config.ALSConfig{}
	t.Cleanup(func() { config.Config = prev })

	f1 := defineMenuCommands()
	f2 := defineMenuCommands()
	if f1 == nil || f2 == nil {
		t.Fatal("factory must not be nil")
	}
	r1 := f1()
	r2 := f2()
	if r1 == r2 {
		t.Error("each factory call should return a different *cobra.Command")
	}
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

func TestDefineMenuCommandsFeatureGating(t *testing.T) {
	tests := []struct {
		name     string
		features *config.ALSConfig
		cmdUse   string
	}{
		{
			name:     "ping disabled",
			features: &config.ALSConfig{FeaturePing: false},
			cmdUse:   "ping",
		},
		{
			name:     "traceroute disabled",
			features: &config.ALSConfig{FeatureTraceroute: false},
			cmdUse:   "traceroute",
		},
		{
			name:     "speedtest disabled",
			features: &config.ALSConfig{FeatureSpeedtestDotNet: false},
			cmdUse:   "speedtest",
		},
		{
			name:     "mtr disabled",
			features: &config.ALSConfig{FeatureMTR: false},
			cmdUse:   "mtr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := config.Config
			config.Config = tt.features
			t.Cleanup(func() { config.Config = prev })

			factory := defineMenuCommands()
			root := factory()

			for _, c := range root.Commands() {
				if c.Use == tt.cmdUse {
					t.Errorf("feature %q should NOT be registered when flag is off", tt.cmdUse)
				}
			}
		})
	}
}
