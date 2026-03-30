// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/nohup implements GNU nohup: run a command immune to hangups.
//
// Implements prd095-nohup R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "nohup"

const nohupOutFile = "nohup.out"

const exitInternal = 125
const exitCannotExec = 126
const exitNotFound = 127

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and executes the nohup logic. Returns exit code.
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return exitInternal
	}
	// R1.1: ignore SIGHUP before executing COMMAND.
	signal.Ignore(syscall.SIGHUP)
	outFile, err := redirectStdout()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return exitInternal
	}
	if outFile != nil {
		defer outFile.Close()
	}
	// R1.3: redirect stderr to stdout if stderr is a terminal and stdout was redirected.
	redirectStderr(outFile)
	// R1.4: pass all remaining arguments to COMMAND.
	return executeCommand(args)
}

// redirectStdout opens nohup.out for appending when stdout is a terminal.
// R1.2: tries current directory first, then $HOME/nohup.out.
func redirectStdout() (*os.File, error) {
	if !sys.IsTerminal(os.Stdout.Fd()) {
		return nil, nil
	}
	f, err := openNohupOut(nohupOutFile)
	if err == nil {
		printRedirectMsg(nohupOutFile)
		os.Stdout = f
		return f, nil
	}
	return redirectToHome()
}

// redirectToHome falls back to $HOME/nohup.out when cwd fails.
func redirectToHome() (*os.File, error) {
	home := os.Getenv("HOME") // platform context: home directory path
	if home == "" {
		return nil, fmt.Errorf("failed to open '%s': Permission denied", nohupOutFile)
	}
	homePath := home + "/" + nohupOutFile
	f, err := openNohupOut(homePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open '%s': %w", homePath, err)
	}
	printRedirectMsg(homePath)
	os.Stdout = f
	return f, nil
}

// openNohupOut opens the nohup.out file for appending with mode 0600.
func openNohupOut(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
}

// printRedirectMsg prints the redirection notice to stderr.
func printRedirectMsg(path string) {
	fmt.Fprintf(os.Stderr, "%s: ignoring input and appending output to '%s'\n",
		progName, path)
}

// redirectStderr redirects stderr to the nohup.out file when stderr is a terminal.
// R1.3: only redirects when stdout was already redirected.
func redirectStderr(outFile *os.File) {
	if outFile == nil || !sys.IsTerminal(os.Stderr.Fd()) {
		return
	}
	os.Stderr = outFile
}

// executeCommand starts the command and returns its exit code.
// R2.1: exits with the exit status of COMMAND.
func executeCommand(args []string) int {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return handleExecError(err, args[0])
	}
	return 0
}

// handleExecError maps command execution errors to exit codes.
// R2.1: propagates COMMAND exit status. R2.2: 125/126/127 for nohup failures.
func handleExecError(err error, cmdName string) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	if isNotFound(err) {
		fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': "+
			"No such file or directory\n", progName, cmdName)
		return exitNotFound
	}
	fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': "+
		"Permission denied\n", progName, cmdName)
	return exitCannotExec
}

// isNotFound checks if the error indicates the command was not found.
func isNotFound(err error) bool {
	return strings.Contains(err.Error(), exec.ErrNotFound.Error())
}
