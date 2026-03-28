// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/nohup implements GNU nohup: run a command immune to hangups.
// Implements prd095-nohup R1.1-R1.4, R2.1-R2.3.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName     = "nohup"
	nohupFile    = "nohup.out"
	exitInternal = 125
	exitNoExec   = 126
	exitNotFound = 127
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run is the main entry point returning the exit code.
// R1.4: args[0] is COMMAND, args[1:] are ARGs passed through.
func run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return exitInternal
	}
	outFile, err := redirectStdout()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return exitInternal
	}
	redirectStderr(outFile)
	// R1.1: ignore SIGHUP before executing COMMAND.
	signal.Ignore(syscall.SIGHUP)
	return execCommand(args[0], args[1:])
}

// printUsage prints usage info to stderr when no COMMAND is given.
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s COMMAND [ARG]...\n", progName)
	fmt.Fprintf(os.Stderr, "Run COMMAND, ignoring hangup signals.\n")
}

// redirectStdout redirects stdout to nohup.out if it is a terminal.
// R1.2: try CWD/nohup.out first, then $HOME/nohup.out.
// D4: O_WRONLY|O_CREATE|O_APPEND, mode 0600.
// D5: uses sys.IsTerminal to check.
func redirectStdout() (*os.File, error) {
	if !sys.IsTerminal(os.Stdout.Fd()) {
		return nil, nil
	}
	f, path, err := openOutputFile()
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "%s: ignoring input and appending output to '%s'\n",
		progName, path)
	if err := dupFD(f, os.Stdout); err != nil {
		f.Close() // best-effort close on error
		return nil, fmt.Errorf("redirecting stdout: %w", err)
	}
	return f, nil
}

// openOutputFile tries nohup.out in CWD, then $HOME/nohup.out.
// Returns the open file and the path used for the message.
func openOutputFile() (*os.File, string, error) {
	f, err := openNohupOut(nohupFile)
	if err == nil {
		return f, nohupFile, nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		return nil, "", fmt.Errorf(
			"failed to open '%s': %v; no HOME directory", nohupFile, err)
	}
	path := filepath.Join(home, nohupFile)
	f, err2 := openNohupOut(path)
	if err2 != nil {
		return nil, "", fmt.Errorf(
			"failed to open '%s' and '%s'", nohupFile, path)
	}
	return f, path, nil
}

// openNohupOut opens the nohup output file with append semantics.
// D4: O_WRONLY|O_CREATE|O_APPEND, mode 0600.
func openNohupOut(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
}

// dupFD duplicates src's file descriptor onto target's file descriptor.
func dupFD(src, target *os.File) error {
	return syscall.Dup2(int(src.Fd()), int(target.Fd()))
}

// redirectStderr redirects stderr to the same file as stdout if stderr
// is a terminal.
// R1.3: if stderr is a terminal, redirect to the nohup.out file.
func redirectStderr(outFile *os.File) {
	if outFile == nil {
		return
	}
	if !sys.IsTerminal(os.Stderr.Fd()) {
		return
	}
	// Dup stdout fd onto stderr fd so both go to nohup.out.
	_ = syscall.Dup2(int(os.Stdout.Fd()), int(os.Stderr.Fd())) // best-effort
}

// execCommand resolves and execs the command, replacing this process.
// D3: use exec.LookPath then syscall.Exec.
// R2.2: exit 127 if not found, 126 if found but not invokable.
func execCommand(name string, args []string) int {
	path, err := exec.LookPath(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': %s\n",
			progName, name, err)
		return exitNotFound
	}
	argv := append([]string{name}, args...)
	err = syscall.Exec(path, argv, os.Environ())
	// If syscall.Exec returns, it failed.
	fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': %s\n",
		progName, name, err)
	return exitNoExec
}
