package config

import (
	"os"
)

// The env helpers are split into a separate file so config_test.go
// stays focused on assertions. They wrap os.Getenv / os.Setenv /
// os.Unsetenv so the test file does not need to import "os" directly.
//
// The //go:noinline directives are intentional: each helper is
// trivial and would be inlined by the compiler otherwise, which
// causes go test -cover to attribute the call's coverage to the
// caller (withEnv) rather than the wrapper. Pinning the noinline
// keeps the wrapper visible in coverage reports as a thin
// shim, and also keeps the surface area unchanged if the body is
// later replaced with a non-trivial implementation.

//go:noinline
func lookup(key string) (string, bool) { return os.LookupEnv(key) }

//go:noinline
func setenv(key, value string) { os.Setenv(key, value) }

//go:noinline
func unsetenv(key string) { os.Unsetenv(key) }
