// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd112-chronic R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	verbose, stderrMode, cmdArgs := parseArgs(args)
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "chronic: usage: chronic [-ev] COMMAND...")
		return 100
	}

	exitCode, stdout, stderr := executeCommand(cmdArgs)

	stderrTriggered := exitCode == 0 && stderrMode && len(stderr) > 0
	if exitCode != 0 || stderrTriggered {
		showOutput(verbose, exitCode, stdout, stderr)
	}

	if stderrTriggered {
		return 2
	}
	return exitCode
}

func parseArgs(args []string) (verbose, stderrMode bool, rest []string) {
	i := 0
	for i < len(args) {
		switch args[i] {
		case "-v", "--verbose":
			verbose = true
		case "-e", "--stderr":
			stderrMode = true
		case "--":
			return verbose, stderrMode, args[i+1:]
		default:
			return verbose, stderrMode, args[i:]
		}
		i++
	}
	return verbose, stderrMode, nil
}

func executeCommand(cmdArgs []string) (int, []byte, []byte) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err == nil {
		return 0, stdoutBuf.Bytes(), stderrBuf.Bytes()
	}

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), stdoutBuf.Bytes(), stderrBuf.Bytes()
	}

	return 100, nil, nil
}

func showOutput(verbose bool, exitCode int, stdout, stderr []byte) {
	if !verbose {
		os.Stdout.Write(stdout)
		os.Stderr.Write(stderr)
		return
	}
	fmt.Fprintf(os.Stderr, "STDOUT:\n")
	os.Stderr.Write(stdout)
	fmt.Fprintf(os.Stderr, "\nSTDERR:\n")
	os.Stderr.Write(stderr)
	fmt.Fprintf(os.Stderr, "\nRETVAL: %d\n", exitCode)
}
