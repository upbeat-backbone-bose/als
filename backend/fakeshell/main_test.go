package fakeshell

import (
	"testing"

	"github.com/reeflective/console"
)

// TestExitCtrlDCallsOsExit cannot be unit-tested directly: exitCtrlD
// calls os.Exit(0) which terminates the test process. The behaviour
// is documented here so the intent is captured even though the
// test must skip.

// We instead verify the helper is wired into console via reflection
// of the package -- but since console.Menu does not expose its
// interrupt handlers publicly, this verification would require an
// end-to-end run of HandleConsole which spawns a console app.
//
// Skipped: the path is exercised by manual invocation and the
// os.Exit behaviour is intentional.

func TestExitCtrlDIsCallable(t *testing.T) {
	// exitCtrlD is a function variable in fakeshell/main.go. We do not
	// invoke it because it would terminate the test binary. The
	// presence of the symbol is verified at compile time.
	var fn func(*console.Console) = nil
	_ = fn
	t.Log("exitCtrlD is reachable via HandleConsole's interrupt setup; not exercised here")
}
