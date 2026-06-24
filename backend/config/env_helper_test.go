package config

import (
	"os"
)

// The env helpers are split into a separate file so config_test.go
// stays focused on assertions. They wrap os.Getenv / os.Setenv /
// os.Unsetenv so the test file does not need to import "os" directly.

//go:noinline
func lookup(key string) (string, bool) { return os.LookupEnv(key) }

//go:noinline
func setenv(key, value string) { os.Setenv(key, value) }

//go:noinline
func unsetenv(key string) { os.Unsetenv(key) }
