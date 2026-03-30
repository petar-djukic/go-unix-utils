// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/timeout implements GNU timeout: run a command with a time limit.
//
// Implements prd063-timeout R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
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

// timeoutConfig holds parsed command-line options.
type timeoutConfig struct {
	signal         syscall.Signal
	killAfter      time.Duration
	foreground     bool
	preserveStatus bool
	duration       time.Duration
	cmdArgs        []string
}

// signalNames maps signal names (without SIG prefix) to signal values.
// R2.1: supports named signal values.
var signalNames = map[string]syscall.Signal{
	"HUP":  syscall.SIGHUP,
	"INT":  syscall.SIGINT,
	"QUIT": syscall.SIGQUIT,
	"ILL":  syscall.SIGILL,
	"TRAP": syscall.SIGTRAP,
	"ABRT": syscall.SIGABRT,
	"BUS":  syscall.SIGBUS,
	"FPE":  syscall.SIGFPE,
	"KILL": syscall.SIGKILL,
	"USR1": syscall.SIGUSR1,
	"SEGV": syscall.SIGSEGV,
	"USR2": syscall.SIGUSR2,
	"PIPE": syscall.SIGPIPE,
	"ALRM": syscall.SIGALRM,
	"TERM": syscall.SIGTERM,
}

func main() {
	sys.InstallSIGPIPEHandler()
	catchTermSignal()
	os.Exit(run(os.Args[1:]))
}

// catchTermSignal prevents SIGTERM from killing the timeout process
// when it is re-raised for exit status propagation to the parent.
func catchTermSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM)
	go func() { for range ch {} }()
}

// run parses arguments and executes the timeout logic. Returns exit code.
func run(args []string) int {
	cfg, exitCode := parseArgs(args)
	if cfg == nil {
		return exitCode
	}
	return executeWithTimeout(cfg)
}

// parseArgs parses command-line arguments into a timeoutConfig.
// Returns nil config and exit code on parse error.
func parseArgs(args []string) (*timeoutConfig, int) {
	cfg := &timeoutConfig{signal: syscall.SIGTERM}
	pos, err := parseFlags(args, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return nil, exitInternal
	}
	return parsePositional(args[pos:], cfg)
}

// parseFlags extracts option flags from args, updating cfg.
// Returns the index of the first positional argument.
func parseFlags(args []string, cfg *timeoutConfig) (int, error) {
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			return i + 1, nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return i, nil
		}
		advance, err := handleFlag(arg, args, i, cfg)
		if err != nil {
			return 0, err
		}
		i += advance
	}
	return i, nil
}

// handleFlag processes a single flag argument.
// Returns how many argument positions to advance.
func handleFlag(arg string, args []string, i int, cfg *timeoutConfig) (int, error) {
	switch {
	case arg == "-s" || arg == "--signal":
		return setSignalFlag(args, i, cfg)
	case strings.HasPrefix(arg, "--signal="):
		sig, err := parseSignal(arg[len("--signal="):])
		if err != nil {
			return 0, err
		}
		cfg.signal = sig
		return 1, nil
	case arg == "-k" || arg == "--kill-after":
		return setKillAfterFlag(args, i, cfg)
	case strings.HasPrefix(arg, "--kill-after="):
		dur, err := parseDuration(arg[len("--kill-after="):])
		if err != nil {
			return 0, err
		}
		cfg.killAfter = dur
		return 1, nil
	case arg == "--foreground":
		cfg.foreground = true
		return 1, nil
	case arg == "--preserve-status":
		cfg.preserveStatus = true
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
}

// setSignalFlag parses the -s / --signal flag with a separate argument.
// R2.1: signal selection.
func setSignalFlag(args []string, i int, cfg *timeoutConfig) (int, error) {
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- 's'")
	}
	sig, err := parseSignal(args[i+1])
	if err != nil {
		return 0, err
	}
	cfg.signal = sig
	return 2, nil
}

// setKillAfterFlag parses the -k / --kill-after flag with a separate argument.
// R2.2: kill-after duration.
func setKillAfterFlag(args []string, i int, cfg *timeoutConfig) (int, error) {
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- 'k'")
	}
	dur, err := parseDuration(args[i+1])
	if err != nil {
		return 0, err
	}
	cfg.killAfter = dur
	return 2, nil
}

// parsePositional parses the DURATION COMMAND [ARG...] positional arguments.
func parsePositional(remaining []string, cfg *timeoutConfig) (*timeoutConfig, int) {
	if len(remaining) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return nil, exitInternal
	}
	if len(remaining) < 2 {
		fmt.Fprintf(os.Stderr, "%s: missing operand after '%s'\n",
			progName, remaining[0])
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return nil, exitInternal
	}
	dur, err := parseDuration(remaining[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return nil, exitInternal
	}
	cfg.duration = dur
	cfg.cmdArgs = remaining[1:]
	return cfg, 0
}

// parseSignal parses a signal name or number string.
// R2.1: accepts signal names (e.g., KILL, HUP) and numeric values.
func parseSignal(s string) (syscall.Signal, error) {
	if n, err := strconv.Atoi(s); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("invalid signal '%s'", s)
		}
		return syscall.Signal(n), nil
	}
	name := strings.TrimPrefix(strings.ToUpper(s), "SIG")
	sig, ok := signalNames[name]
	if !ok {
		return 0, fmt.Errorf("invalid signal '%s'", s)
	}
	return sig, nil
}

// executeWithTimeout runs the command with an optional time limit.
// R1.1: kills with configured signal if command does not exit within duration.
// R2.3: skips process group creation when foreground is true.
func executeWithTimeout(cfg *timeoutConfig) int {
	cmd := exec.Command(cfg.cmdArgs[0], cfg.cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !cfg.foreground {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if err := cmd.Start(); err != nil {
		return handleStartError(err, cfg.cmdArgs[0])
	}
	return waitWithTimeout(cmd, cfg)
}

// handleStartError maps command start errors to exit codes.
func handleStartError(err error, cmdName string) int {
	fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': %s\n",
		progName, cmdName, err)
	if isNotFound(err) {
		return exitNotFound
	}
	return exitCannotExec
}

// isNotFound checks if the error indicates the command was not found.
func isNotFound(err error) bool {
	return strings.Contains(err.Error(), exec.ErrNotFound.Error())
}

// waitWithTimeout waits for the command to complete or handles timeout.
// R1.4: duration 0 means no time limit.
func waitWithTimeout(cmd *exec.Cmd, cfg *timeoutConfig) int {
	if cfg.duration == 0 {
		return waitForExit(cmd)
	}
	timer := time.NewTimer(cfg.duration)
	defer timer.Stop()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-timer.C:
		return handleTimeout(cmd, cfg, done)
	case err := <-done:
		return handleCommandDone(err)
	}
}

// handleTimeout sends the configured signal and waits for the command.
// R2.2: escalates to SIGKILL after killAfter duration.
// R2.4: preserves command exit status when preserveStatus is true.
func handleTimeout(cmd *exec.Cmd, cfg *timeoutConfig, done chan error) int {
	signalProcess(cmd, cfg.signal, cfg.foreground)
	waitErr := waitOrEscalate(cmd, cfg, done)
	if cfg.preserveStatus {
		return exitCodeFromErr(waitErr)
	}
	// Re-raise the configured signal on self to propagate signal status
	// to the parent process. For uncatchable signals like SIGKILL, this
	// kills the timeout process (matching GNU timeout behavior). For
	// SIGTERM, our catchTermSignal handler absorbs it and we exit 124.
	reraiseSignal(cfg.signal)
	return exitTimeout
}

// reraiseSignal sends the specified signal to the current process.
// For catchable signals, a temporary handler absorbs the signal so the
// process can continue and return exit code 124. For uncatchable signals
// (SIGKILL, SIGSTOP), the process terminates immediately.
func reraiseSignal(sig syscall.Signal) {
	if sig == syscall.SIGKILL || sig == syscall.SIGSTOP {
		syscall.Kill(os.Getpid(), sig)
		time.Sleep(time.Second) // unreachable; kernel terminates us
		return
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sig)
	syscall.Kill(os.Getpid(), sig)
	<-ch // drain the caught signal
	signal.Stop(ch)
}

// signalProcess sends a signal to the command process or its process group.
// R2.3: signals the process directly when foreground is true.
func signalProcess(cmd *exec.Cmd, sig syscall.Signal, foreground bool) {
	if foreground {
		_ = cmd.Process.Signal(sig) // best-effort
		return
	}
	// best-effort signal to process group
	_ = syscall.Kill(-cmd.Process.Pid, sig)
}

// waitOrEscalate waits for the command to exit, escalating to SIGKILL
// if killAfter is configured and the command does not exit in time.
// R2.2: kill-after escalation.
func waitOrEscalate(cmd *exec.Cmd, cfg *timeoutConfig, done chan error) error {
	if cfg.killAfter == 0 {
		return <-done
	}
	killTimer := time.NewTimer(cfg.killAfter)
	defer killTimer.Stop()
	select {
	case err := <-done:
		return err
	case <-killTimer.C:
		signalProcess(cmd, syscall.SIGKILL, cfg.foreground)
		return <-done
	}
}

// exitCodeFromErr extracts the exit code from a command Wait error.
// Returns 128+signum for signaled processes per shell convention.
func exitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return exitInternal
	}
	ws, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
		return exitInternal
	}
	if ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ws.ExitStatus()
}

// waitForExit waits for the command to complete and returns its exit code.
// R3.3: re-raises signal if the child was killed by one.
func waitForExit(cmd *exec.Cmd) int {
	return handleCommandDone(cmd.Wait())
}

// handleCommandDone processes a command's exit result, re-raising any signal
// the child was killed by to propagate signal death to the parent process.
// R3.3: when the command is killed by a signal not sent by timeout, must
// match GNU timeout behavior of re-raising the signal on self.
func handleCommandDone(err error) int {
	code := exitCodeFromErr(err)
	if sig := childSignal(err); sig != 0 {
		reraiseChildSignal(sig)
	}
	return code
}

// childSignal extracts the signal that killed the child process, if any.
func childSignal(err error) syscall.Signal {
	if err == nil {
		return 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return 0
	}
	ws, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return 0
	}
	if ws.Signaled() {
		return ws.Signal()
	}
	return 0
}

// reraiseChildSignal resets Go signal handling to default and re-raises
// the signal, matching GNU timeout's signal propagation behavior.
func reraiseChildSignal(sig syscall.Signal) {
	signal.Reset(sig)
	syscall.Kill(os.Getpid(), sig)
	time.Sleep(time.Millisecond)
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
