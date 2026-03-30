// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/timeout implements GNU timeout: run a command with a time limit.
//
// Implements prd063-timeout R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "timeout"

const exitTimeout = 124
const exitInternal = 125
const exitCannotExec = 126
const exitNotFound = 127

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run implements the timeout logic. Returns exit code.
func run(args []string) int {
	if len(args) < 2 {
		printUsageError(args)
		return exitInternal
	}

	durationStr := args[0]
	cmdArgs := args[1:]

	dur, err := parseDuration(durationStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return exitInternal
	}

	return executeWithTimeout(dur, cmdArgs)
}

// printUsageError prints the missing-operand error to stderr.
func printUsageError(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
	} else {
		fmt.Fprintf(os.Stderr, "%s: missing operand after '%s'\n", progName, args[0])
	}
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

// executeWithTimeout runs the command with an optional time limit.
// R1.1: kills with SIGTERM if command does not exit within duration.
// R1.4: duration 0 means no time limit.
func executeWithTimeout(dur time.Duration, cmdArgs []string) int {
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return handleStartError(err, cmdArgs[0])
	}

	return waitWithTimeout(cmd, dur)
}

// handleStartError maps command start errors to exit codes.
func handleStartError(err error, cmdName string) int {
	fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': %s\n", progName, cmdName, err)
	if isNotFound(err) {
		return exitNotFound
	}
	return exitCannotExec
}

// isNotFound checks if the error indicates the command was not found.
func isNotFound(err error) bool {
	return strings.Contains(err.Error(), exec.ErrNotFound.Error())
}

// waitWithTimeout waits for the command to complete or kills it on timeout.
func waitWithTimeout(cmd *exec.Cmd, dur time.Duration) int {
	// R1.4: duration 0 means no time limit.
	if dur == 0 {
		return waitForExit(cmd)
	}

	timer := time.NewTimer(dur)
	defer timer.Stop()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-timer.C:
		return killAndWait(cmd, done)
	case err := <-done:
		return exitCodeFromErr(err)
	}
}

// killAndWait sends SIGTERM to the process group and waits for the
// command to exit. R1.1: kills with SIGTERM on timeout.
func killAndWait(cmd *exec.Cmd, done chan error) int {
	// Send SIGTERM to the process group.
	pgid := -cmd.Process.Pid
	// best-effort signal to process group
	_ = syscall.Kill(pgid, syscall.SIGTERM)

	<-done
	return exitTimeout
}

// waitForExit waits for the command to complete and returns its exit code.
func waitForExit(cmd *exec.Cmd) int {
	err := cmd.Wait()
	return exitCodeFromErr(err)
}

// exitCodeFromErr extracts the exit code from an exec error.
func exitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return exitInternal
}

// parseDuration parses a duration string with optional suffix.
// R1.2: supports fractional values (e.g., 0.5).
// R1.3: supports suffix multipliers s, m, h, d.
func parseDuration(s string) (time.Duration, error) {
	multiplier := 1.0
	numStr := s

	if len(s) > 0 {
		last := s[len(s)-1]
		if m, ok := suffixMultiplier(last); ok {
			multiplier = m
			numStr = s[:len(s)-1]
		}
	}

	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil || val < 0 {
		return 0, fmt.Errorf("invalid time interval '%s'", s)
	}

	// R1.4: duration 0 means no time limit.
	if val == 0 {
		return 0, nil
	}

	secs := val * multiplier
	return time.Duration(secs * float64(time.Second)), nil
}

// suffixMultiplier returns the multiplier for a duration suffix.
// R1.3: s (seconds), m (minutes), h (hours), d (days).
func suffixMultiplier(ch byte) (float64, bool) {
	switch ch {
	case 's':
		return 1, true
	case 'm':
		return 60, true
	case 'h':
		return 3600, true
	case 'd':
		return 86400, true
	default:
		return 0, false
	}
}
