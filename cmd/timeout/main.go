// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/timeout: run a command with a time limit.
// Implements srd063-timeout R1.1, R1.2, R1.3, R1.4.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "timeout"

const (
	exitTimeout  = 124
	exitInternal = 125
	exitNotExec  = 126
	exitNotFound = 127
)

// suffixMultipliers maps duration suffix characters to their
// multiplier in seconds. R1.3: s, m, h, d suffixes.
var suffixMultipliers = map[byte]float64{
	's': 1,
	'm': 60,
	'h': 3600,
	'd': 86400,
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and executes the timeout command.
// R1.1: DURATION COMMAND [ARG...] form.
func run(args []string) int {
	if len(args) < 2 {
		printUsageError()
		return exitInternal
	}
	dur, err := parseDuration(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: invalid time interval '%s'\n",
			progName, args[0])
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n",
			progName)
		return exitInternal
	}
	return executeCommand(dur, args[1], args[2:])
}

// printUsageError writes the try-help message to stderr.
func printUsageError() {
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

// parseDuration parses a duration string with optional suffix.
// R1.2: fractional values. R1.3: suffix multipliers s, m, h, d.
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	multiplier := 1.0
	numStr := s
	last := s[len(s)-1]
	if m, ok := suffixMultipliers[last]; ok {
		multiplier = m
		numStr = s[:len(s)-1]
	}
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil || val < 0 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}
	seconds := val * multiplier
	return time.Duration(seconds * float64(time.Second)), nil
}

// executeCommand starts the command and applies the timeout.
// R1.1: kill with SIGTERM on timeout. R1.4: duration 0 means no limit.
func executeCommand(dur time.Duration, name string, args []string) int {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return handleStartError(err, name)
	}
	if dur == 0 {
		return waitForCommand(cmd)
	}
	return waitWithTimeout(cmd, dur)
}

// waitWithTimeout waits for the command or kills it on timeout.
// R1.1: sends SIGTERM to the process group when the timer fires.
func waitWithTimeout(cmd *exec.Cmd, dur time.Duration) int {
	done := make(chan int, 1)
	go func() {
		done <- waitForCommand(cmd)
	}()
	timer := time.NewTimer(dur)
	defer timer.Stop()
	select {
	case code := <-done:
		return code
	case <-timer.C:
		killProcessGroup(cmd.Process.Pid)
		<-done
		return exitTimeout
	}
}

// waitForCommand waits for the command to finish and returns its exit code.
// Handles both normal exit and signal-killed processes.
func waitForCommand(cmd *exec.Cmd) int {
	err := cmd.Wait()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		ws, ok := exitErr.Sys().(syscall.WaitStatus)
		if ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return exitErr.ExitCode()
	}
	return exitInternal
}

// handleStartError returns the appropriate exit code for a failed exec.
func handleStartError(err error, name string) int {
	if errors.Is(err, exec.ErrNotFound) {
		fmt.Fprintf(os.Stderr,
			"%s: failed to run command '%s': No such file or directory\n",
			progName, name)
		return exitNotFound
	}
	fmt.Fprintf(os.Stderr,
		"%s: failed to run command '%s': Permission denied\n",
		progName, name)
	return exitNotExec
}

// killProcessGroup sends SIGTERM to the entire process group.
func killProcessGroup(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGTERM) // best-effort signal delivery
}
