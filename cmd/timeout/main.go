// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd063-timeout R1.1-R1.4 (core timeout behavior),
// R2.1-R2.4 (signal selection, kill-after, foreground, preserve-status),
// R3.1-R3.4 (exit codes: command status, timeout 124, signal 128+N, errors 125-127),
// and R4.1-R4.4 (differential testing, orphan cleanup verification).
package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
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

  -s, --signal=SIGNAL    specify the signal to be sent on timeout;
                           SIGNAL may be a name like 'HUP' or a number;
                           see 'kill -l' for a list of signals
  -k, --kill-after=DURATION
                         also send a KILL signal if COMMAND is still running
                           this long after the initial signal was sent
      --foreground       when not running timeout directly from a shell prompt,
                           allow COMMAND to read from the TTY and get TTY signals;
                           in this mode, children of COMMAND will not be timed out
      --preserve-status  exit with the same status as COMMAND, even when the
                           command times out
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "timeout (go-unix-utils) 1.0\n"

// signalMap maps uppercase signal names (without SIG prefix) to signals.
var signalMap = map[string]syscall.Signal{
	"HUP":  syscall.SIGHUP,
	"INT":  syscall.SIGINT,
	"QUIT": syscall.SIGQUIT,
	"ILL":  syscall.SIGILL,
	"TRAP": syscall.SIGTRAP,
	"ABRT": syscall.SIGABRT,
	"FPE":  syscall.SIGFPE,
	"KILL": syscall.SIGKILL,
	"BUS":  syscall.SIGBUS,
	"SEGV": syscall.SIGSEGV,
	"PIPE": syscall.SIGPIPE,
	"ALRM": syscall.SIGALRM,
	"TERM": syscall.SIGTERM,
	"URG":  syscall.SIGURG,
	"STOP": syscall.SIGSTOP,
	"TSTP": syscall.SIGTSTP,
	"CONT": syscall.SIGCONT,
	"CHLD": syscall.SIGCHLD,
	"USR1": syscall.SIGUSR1,
	"USR2": syscall.SIGUSR2,
}

// timeoutConfig holds parsed command-line options.
type timeoutConfig struct {
	signal         syscall.Signal
	killAfter      time.Duration
	foreground     bool
	preserveStatus bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	if len(args) == 0 {
		exitMissing("")
	}
	cfg, positional := parseAllArgs(args)
	if len(positional) == 0 {
		exitMissing("")
	}
	if len(positional) < 2 {
		exitMissing(positional[0])
	}
	dur, err := parseDuration(positional[0])
	if err != nil {
		exitInvalidInterval(positional[0])
	}
	os.Exit(runWithTimeout(dur, positional[1], positional[2:], cfg))
}

// parseAllArgs separates options from positional arguments.
func parseAllArgs(args []string) (*timeoutConfig, []string) {
	cfg := &timeoutConfig{signal: syscall.SIGTERM}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if !isOption(arg) {
			break
		}
		i += handleOption(cfg, args, i)
	}
	return cfg, args[i:]
}

// isOption returns true if the argument looks like a command-line option.
func isOption(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	if s[1] == '-' {
		return true
	}
	return (s[1] >= 'a' && s[1] <= 'z') || (s[1] >= 'A' && s[1] <= 'Z')
}

// handleOption dispatches a single option and returns the number of args consumed.
func handleOption(cfg *timeoutConfig, args []string, i int) int {
	arg := args[i]
	switch arg {
	case "--help":
		fmt.Print(helpText)
		os.Exit(0)
	case "--version":
		fmt.Print(versionText)
		os.Exit(0)
	}
	if strings.HasPrefix(arg, "--") {
		return handleLongOption(cfg, args, i)
	}
	return handleShortOption(cfg, args, i)
}

// handleLongOption parses a long option (--signal, --kill-after, etc.).
func handleLongOption(cfg *timeoutConfig, args []string, i int) int {
	arg := args[i]
	switch {
	case arg == "--foreground":
		cfg.foreground = true
		return 1
	case arg == "--preserve-status":
		cfg.preserveStatus = true
		return 1
	case arg == "--signal", strings.HasPrefix(arg, "--signal="):
		return handleSignalFlag(cfg, arg, args, i)
	case arg == "--kill-after", strings.HasPrefix(arg, "--kill-after="):
		return handleKillAfterFlag(cfg, arg, args, i)
	default:
		exitUnknownOption(arg)
		return 0
	}
}

// handleSignalFlag parses --signal=VALUE or --signal VALUE. R2.1.
func handleSignalFlag(cfg *timeoutConfig, arg string, args []string, i int) int {
	val, consumed := extractLongValue(arg, "--signal=", args, i)
	sig, err := parseSignal(val)
	if err != nil {
		exitInvalidSignal(val)
	}
	cfg.signal = sig
	return consumed
}

// handleKillAfterFlag parses --kill-after=VALUE or --kill-after VALUE. R2.2.
func handleKillAfterFlag(cfg *timeoutConfig, arg string, args []string, i int) int {
	val, consumed := extractLongValue(arg, "--kill-after=", args, i)
	dur, err := parseDuration(val)
	if err != nil {
		exitInvalidInterval(val)
	}
	cfg.killAfter = dur
	return consumed
}

// extractLongValue extracts the value from a --key=value or --key value form.
func extractLongValue(arg, eqPrefix string, args []string, i int) (string, int) {
	if strings.HasPrefix(arg, eqPrefix) {
		return arg[len(eqPrefix):], 1
	}
	if i+1 >= len(args) {
		exitMissingOptArg(arg)
	}
	return args[i+1], 2
}

// handleShortOption parses a short option (-s, -k).
func handleShortOption(cfg *timeoutConfig, args []string, i int) int {
	arg := args[i]
	switch arg[1] {
	case 's':
		val, consumed := extractShortValue(arg, args, i)
		sig, err := parseSignal(val)
		if err != nil {
			exitInvalidSignal(val)
		}
		cfg.signal = sig
		return consumed
	case 'k':
		val, consumed := extractShortValue(arg, args, i)
		dur, err := parseDuration(val)
		if err != nil {
			exitInvalidInterval(val)
		}
		cfg.killAfter = dur
		return consumed
	default:
		exitUnknownOption(arg)
		return 0
	}
}

// extractShortValue extracts the value from -Xvalue or -X value form.
func extractShortValue(arg string, args []string, i int) (string, int) {
	if len(arg) > 2 {
		return arg[2:], 1
	}
	if i+1 >= len(args) {
		exitMissingOptArg(arg)
	}
	return args[i+1], 2
}

// parseSignal parses a signal name or number string.
// R2.1: accepts signal names (KILL, HUP, SIGKILL) and numeric values.
func parseSignal(s string) (syscall.Signal, error) {
	if num, err := strconv.Atoi(s); err == nil {
		if num > 0 {
			return syscall.Signal(num), nil
		}
		return 0, fmt.Errorf("invalid signal %q", s)
	}
	name := strings.TrimPrefix(s, "SIG")
	if sig, ok := signalMap[name]; ok {
		return sig, nil
	}
	return 0, fmt.Errorf("invalid signal %q", s)
}

// runWithTimeout executes a command and kills it if it exceeds dur.
// R1.1: SIGTERM on timeout. R1.4: dur==0 means no limit.
// R3.3: when the child dies by signal before timeout, re-raises the signal.
func runWithTimeout(dur time.Duration, command string, args []string, cfg *timeoutConfig) int {
	cmd := buildCommand(command, args, cfg.foreground)
	if err := cmd.Start(); err != nil {
		return handleStartError(err, command)
	}
	if dur == 0 {
		return exitCodeFromWait(cmd.Wait(), true)
	}
	return waitWithTimeout(cmd, dur, cfg)
}

// buildCommand creates an exec.Cmd with stdio connected.
// R2.3: --foreground skips Setpgid so the child shares the terminal.
func buildCommand(command string, args []string, foreground bool) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !foreground {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	return cmd
}

// waitWithTimeout waits for cmd to finish or kills it after dur.
// R3.3: child dying by signal before timeout triggers re-raise.
func waitWithTimeout(cmd *exec.Cmd, dur time.Duration, cfg *timeoutConfig) int {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(dur)
	defer timer.Stop()
	select {
	case err := <-done:
		return exitCodeFromWait(err, true)
	case <-timer.C:
		sendSignalToCmd(cmd, cfg.signal, cfg.foreground)
		return handleTimeoutExit(cmd, done, cfg)
	}
}

// handleTimeoutExit waits for the child after sending the initial signal.
// R2.2: if killAfter > 0, starts a kill-after escalation timer.
func handleTimeoutExit(cmd *exec.Cmd, done <-chan error, cfg *timeoutConfig) int {
	if cfg.killAfter > 0 {
		return waitWithKillAfter(done, cmd, cfg)
	}
	waitErr := <-done
	return resolveTimeoutExitCode(waitErr, cfg.preserveStatus)
}

// waitWithKillAfter waits for the child to exit, escalating to SIGKILL
// if the child is still running after killAfter. R2.2.
func waitWithKillAfter(done <-chan error, cmd *exec.Cmd, cfg *timeoutConfig) int {
	killTimer := time.NewTimer(cfg.killAfter)
	defer killTimer.Stop()
	select {
	case waitErr := <-done:
		return resolveTimeoutExitCode(waitErr, cfg.preserveStatus)
	case <-killTimer.C:
		sendSignalToCmd(cmd, syscall.SIGKILL, cfg.foreground)
		waitErr := <-done
		return resolveTimeoutExitCode(waitErr, cfg.preserveStatus)
	}
}

// resolveTimeoutExitCode returns the appropriate exit code after a timeout.
// R2.4: --preserve-status returns the command's actual exit status.
// R3.2: without --preserve-status, returns 124.
func resolveTimeoutExitCode(waitErr error, preserveStatus bool) int {
	if preserveStatus {
		return exitCodeFromWait(waitErr, false)
	}
	return exitTimeout
}

// sendSignalToCmd sends a signal to the command process.
// When foreground is false, sends to the child's process group via negative pid.
// For SIGKILL, also sends to ourselves to match GNU timeout behavior where
// timeout and child share a process group and both die on SIGKILL.
func sendSignalToCmd(cmd *exec.Cmd, sig syscall.Signal, foreground bool) {
	pid := cmd.Process.Pid
	if foreground {
		syscall.Kill(pid, sig) //nolint:errcheck // process may have exited
		return
	}
	syscall.Kill(-pid, sig) //nolint:errcheck // process may have exited
	if sig == syscall.SIGKILL {
		syscall.Kill(os.Getpid(), syscall.SIGKILL) //nolint:errcheck // intentional self-kill
	}
}

// exitCodeFromWait extracts the exit code from a cmd.Wait() error.
// R3.1: normal exits return the command's exit code.
// R3.3: when reraise is true and the child was killed by a signal,
// re-raises the signal on ourselves to match GNU timeout behavior.
func exitCodeFromWait(err error, reraise bool) int {
	if err == nil {
		return 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return exitInternalError
	}
	code := exitErr.ExitCode()
	if code >= 0 {
		return code
	}
	return signalExitCode(exitErr, reraise)
}

// signalExitCode extracts 128+signum from a signal-killed process.
// R3.3: when reraise is true, re-raises the child's signal on ourselves
// so the parent process sees the correct signal death, matching GNU timeout.
func signalExitCode(exitErr *exec.ExitError, reraise bool) int {
	ws, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok || !ws.Signaled() {
		return exitInternalError
	}
	sig := ws.Signal()
	if reraise {
		reraiseSignal(sig)
	}
	return 128 + int(sig)
}

// reraiseSignal resets the signal handler to default and sends the signal
// to ourselves, causing the process to die with the same signal as the child.
// R3.3: matches GNU timeout behavior of propagating the child's signal death.
// Blocks briefly to allow the runtime to deliver the signal before returning.
func reraiseSignal(sig syscall.Signal) {
	signal.Reset(sig)
	syscall.Kill(os.Getpid(), sig) //nolint:errcheck // intentional self-signal
	// Give the runtime time to deliver the fatal signal. For signals
	// whose default action is terminate, the process dies during this
	// pause. Falls through to 128+signum fallback otherwise.
	time.Sleep(time.Second)
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

// exitInvalidSignal prints an invalid signal error and exits 125.
func exitInvalidSignal(s string) {
	fmt.Fprintf(os.Stderr, "%s: invalid signal %q\n", programName, s)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n",
		programName)
	os.Exit(exitInternalError)
}

// exitUnknownOption prints an unrecognized option error and exits 125.
func exitUnknownOption(opt string) {
	fmt.Fprintf(os.Stderr, "%s: unrecognized option %q\n", programName, opt)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n",
		programName)
	os.Exit(exitInternalError)
}

// exitMissingOptArg prints a missing option argument error and exits 125.
func exitMissingOptArg(opt string) {
	fmt.Fprintf(os.Stderr, "%s: option %q requires an argument\n",
		programName, opt)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n",
		programName)
	os.Exit(exitInternalError)
}
