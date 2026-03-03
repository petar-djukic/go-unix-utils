// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// testbin is a configurable helper binary for unit-testing the differential
// testing harness in pkg/testutils. Flags control stdout content, stderr
// content, and exit code. Default values may be injected via -ldflags to
// produce divergent binaries from a single source file.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

// Defaults overridable via -ldflags "-X main.defaultStdout=value".
var (
	defaultStdout   string
	defaultStderr   string
	defaultExitCode string
)

func main() {
	defExit := 0
	if defaultExitCode != "" {
		defExit, _ = strconv.Atoi(defaultExitCode) // best-effort parse
	}

	stdoutVal := flag.String("stdout", defaultStdout, "content to write to stdout")
	stderrVal := flag.String("stderr", defaultStderr, "content to write to stderr")
	exitCodeVal := flag.Int("exit-code", defExit, "exit code to return")
	flag.Parse()

	fmt.Fprint(os.Stdout, *stdoutVal)
	fmt.Fprint(os.Stderr, *stderrVal)
	os.Exit(*exitCodeVal)
}
