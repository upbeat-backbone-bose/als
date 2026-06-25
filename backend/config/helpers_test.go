package config

import "testing"

// withConfig swaps the global Config for the duration of t,
// restoring the original pointer on cleanup. Use this in any
// test that mutates Config so a panic or early return cannot
// leak state into the next test.
func withConfig(t *testing.T, cfg *ALSConfig) {
	t.Helper()
	prev := Config
	Config = cfg
	t.Cleanup(func() { Config = prev })
}

// withInternalCall sets IsInternalCall to value and restores the
// previous value on cleanup.
func withInternalCall(t *testing.T, value bool) {
	t.Helper()
	prev := IsInternalCall
	IsInternalCall = value
	t.Cleanup(func() { IsInternalCall = prev })
}
