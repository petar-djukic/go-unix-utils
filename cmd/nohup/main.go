// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd095-nohup R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "nohup: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'nohup --help' for more information.")
		os.Exit(125)
	}

	signal.Ignore(syscall.SIGHUP)

	outFile := redirectStdout()
	redirectStderr(outFile)

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		exitFromError(err, os.Args[1])
	}
}

func redirectStdout() *os.File {
	if !sys.IsTerminal(os.Stdout.Fd()) {
		return nil
	}
	f, err := openNohupOut()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nohup: failed to open 'nohup.out': %s\n", err)
		os.Exit(125)
	}
	os.Stdout = f
	return f
}

func openNohupOut() (*os.File, error) {
	f, err := os.OpenFile("nohup.out", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err == nil {
		fmt.Fprintln(os.Stderr, "nohup: ignoring input and appending output to 'nohup.out'")
		return f, nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		return nil, err
	}
	path := home + "/nohup.out"
	f, err2 := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err2 != nil {
		return nil, err2
	}
	fmt.Fprintf(os.Stderr, "nohup: ignoring input and appending output to '%s'\n", path)
	return f, nil
}

func redirectStderr(outFile *os.File) {
	if outFile == nil || !sys.IsTerminal(os.Stderr.Fd()) {
		return
	}
	os.Stderr = outFile
}

func exitFromError(err error, command string) {
	if exitErr, ok := err.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode())
	}
	if _, lookErr := exec.LookPath(command); lookErr != nil {
		fmt.Fprintf(os.Stderr, "nohup: failed to run command '%s': No such file or directory\n", command)
		os.Exit(127)
	}
	fmt.Fprintf(os.Stderr, "nohup: failed to run command '%s': %s\n", command, err)
	os.Exit(126)
}
