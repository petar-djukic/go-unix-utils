// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/nohup: run a command immune to hangups.
// Implements srd095-nohup R1.1-R1.4, R2.1-R2.3.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "nohup"

const (
	exitInternal = 125
	exitNotExec  = 126
	exitNotFound = 127
)

const nohupOutFile = "nohup.out"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the nohup command logic and returns an exit code.
// R1.1-R1.4, R2.1-R2.3.
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return exitInternal
	}

	// R1.1: ignore SIGHUP before executing the command.
	signal.Ignore(syscall.SIGHUP)

	outFile, code := redirectStdout()
	if code != 0 {
		return code
	}
	if outFile != nil {
		defer outFile.Close() // best-effort close
	}

	// R1.3: if stderr is a terminal, redirect to the same file as stdout.
	redirectStderr(outFile)

	return executeCommand(args, outFile)
}

// redirectStdout opens nohup.out if stdout is a terminal.
// R1.2: try CWD first, then $HOME/nohup.out.
func redirectStdout() (*os.File, int) {
	if !sys.IsTerminal(os.Stdout.Fd()) {
		return nil, 0
	}

	f, err := openNohupOut(nohupOutFile)
	if err == nil {
		fmt.Fprintf(os.Stderr, "%s: appending output to '%s'\n", progName, nohupOutFile)
		return f, 0
	}

	home := os.Getenv("HOME") // platform context: HOME path
	if home == "" {
		fmt.Fprintf(os.Stderr, "%s: failed to open '%s': %v\n", progName, nohupOutFile, err)
		return nil, exitInternal
	}

	homePath := filepath.Join(home, nohupOutFile)
	f, err = openNohupOut(homePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to open '%s': %v\n", progName, homePath, err)
		return nil, exitInternal
	}

	fmt.Fprintf(os.Stderr, "%s: appending output to '%s'\n", progName, homePath)
	return f, 0
}

// openNohupOut opens a file for appending with mode 0600.
func openNohupOut(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
}

// redirectStderr redirects stderr to outFile if stderr is a terminal.
// R1.3: stderr goes to the same destination as redirected stdout.
func redirectStderr(outFile *os.File) {
	if outFile == nil || !sys.IsTerminal(os.Stderr.Fd()) {
		return
	}
	// Redirect stderr to the same file by dup2.
	_ = syscall.Dup2(int(outFile.Fd()), int(os.Stderr.Fd())) // best-effort: if dup2 fails, stderr stays on terminal
}

// executeCommand runs the command and returns the appropriate exit code.
// R1.4: pass all arguments. R2.1-R2.2: propagate child exit code.
func executeCommand(args []string, outFile *os.File) int {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	if outFile != nil {
		cmd.Stdout = outFile
	} else {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err == nil {
		return 0
	}
	return classifyError(err)
}

// classifyError maps exec errors to nohup exit codes.
// R2.2: 125 internal, 126 not executable, 127 not found.
func classifyError(err error) int {
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}
	if errors.Is(err, exec.ErrNotFound) {
		fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': No such file or directory\n", progName, os.Args[1])
		return exitNotFound
	}
	// Check for permission error (found but not executable).
	if isPermissionError(err) {
		fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': Permission denied\n", progName, os.Args[1])
		return exitNotExec
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
	return exitInternal
}

// isPermissionError checks if the error indicates a permission denial.
func isPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES)
}
