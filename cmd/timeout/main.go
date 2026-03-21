// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd063-timeout R1.1-R1.4: core timeout behavior including
// command execution with time limit, fractional durations, suffix multipliers,
// and zero-duration bypass.
package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "timeout"

// Exit codes per GNU timeout convention.
const (
	exitTimeout       = 124
	exitInternalError = 125
	exitCannotExec    = 126
	exitNotFound      = 127
)

const helpText = `Usage: timeout [OPTION] DURATION COMMAND [ARG]...
  or:  timeout [OPTION]
Start COMMAND, and kill it if still running after DURATION.

DURATION is a floating point number with an optional suffix:
's' for seconds (the default), 'm' for minutes, 'h' for hours or 'd' for days.
A duration of 0 disables the associated timeout.

      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "timeout (go-unix-utils) 1.0\n"

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	if len(args) == 0 {
		exitMissing("")
	}
	handleSpecialFlags(args[0])
	if len(args) < 2 {
		exitMissing(args[0])
	}
	dur, err := parseDuration(args[0])
	if err != nil {
		exitInvalidInterval(args[0])
	}
	os.Exit(runWithTimeout(dur, args[1], args[2:]))
}

// handleSpecialFlags checks for --help and --version, exiting if found.
func handleSpecialFlags(arg string) {
	switch arg {
	case "--help":
		fmt.Print(helpText)
		os.Exit(0)
	case "--version":
		fmt.Print(versionText)
		os.Exit(0)
	}
}

// runWithTimeout executes a command and kills it if it exceeds dur.
// R1.1: SIGTERM on timeout. R1.4: dur==0 means no limit.
func runWithTimeout(dur time.Duration, command string, args []string) int {
	cmd := buildCommand(command, args)
	if err := cmd.Start(); err != nil {
		return handleStartError(err, command)
	}
	if dur == 0 {
		return exitCodeFromError(cmd.Wait())
	}
	return waitWithTimeout(cmd, dur)
}

// buildCommand creates an exec.Cmd with stdio connected and a new
// process group via Setpgid.
func buildCommand(command string, args []string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// waitWithTimeout waits for cmd to finish or kills it after dur.
// R1.1: sends SIGTERM to the process group on timeout.
func waitWithTimeout(cmd *exec.Cmd, dur time.Duration) int {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(dur)
	defer timer.Stop()
	select {
	case err := <-done:
		return exitCodeFromError(err)
	case <-timer.C:
		killProcessGroup(cmd.Process.Pid)
		<-done
		return exitTimeout
	}
}

// killProcessGroup sends SIGTERM to the process group led by pid.
func killProcessGroup(pid int) {
	// Negative pid targets the entire process group; best-effort.
	syscall.Kill(-pid, syscall.SIGTERM) //nolint:errcheck // process may have exited
}

// exitCodeFromError extracts the exit code from a cmd.Wait() error.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return exitInternalError
}

// handleStartError maps exec start failures to exit codes.
// R3.4: 127 for not found, 126 for cannot execute.
func handleStartError(err error, command string) int {
	fmt.Fprintf(os.Stderr, "%s: failed to run command %q: %v\n",
		programName, command, err)
	if isExecNotFound(err) {
		return exitNotFound
	}
	if os.IsPermission(err) {
		return exitCannotExec
	}
	return exitInternalError
}

// isExecNotFound returns true if err indicates command was not found.
func isExecNotFound(err error) bool {
	_, ok := err.(*exec.Error)
	return ok
}

// parseDuration parses a duration string with optional suffix.
// R1.2: fractional values. R1.3: suffix multipliers s, m, h, d.
func parseDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration")
	}
	multiplier, numStr := extractSuffix(s)
	seconds, err := strconv.ParseFloat(numStr, 64)
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("invalid time interval %q", s)
	}
	return clampDuration(seconds * multiplier), nil
}

// extractSuffix splits a duration string into multiplier and numeric part.
func extractSuffix(s string) (float64, string) {
	switch s[len(s)-1] {
	case 's':
		return 1.0, s[:len(s)-1]
	case 'm':
		return 60.0, s[:len(s)-1]
	case 'h':
		return 3600.0, s[:len(s)-1]
	case 'd':
		return 86400.0, s[:len(s)-1]
	default:
		return 1.0, s
	}
}

// clampDuration converts seconds to time.Duration, clamping on overflow.
func clampDuration(seconds float64) time.Duration {
	maxSec := float64(math.MaxInt64) / float64(time.Second)
	if math.IsInf(seconds, 1) || seconds > maxSec {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(seconds * float64(time.Second))
}

// exitMissing prints a missing operand error and exits 125.
func exitMissing(after string) {
	if after == "" {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
	} else {
		fmt.Fprintf(os.Stderr, "%s: missing operand after %q\n",
			programName, after)
	}
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n",
		programName)
	os.Exit(exitInternalError)
}

// exitInvalidInterval prints an invalid interval error and exits 125.
func exitInvalidInterval(s string) {
	fmt.Fprintf(os.Stderr, "%s: invalid time interval %q\n", programName, s)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n",
		programName)
	os.Exit(exitInternalError)
}
