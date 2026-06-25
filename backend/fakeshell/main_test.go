package fakeshell

import (
	"testing"

	"github.com/reeflective/console"
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
