// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/timeout: run a command with a time limit.
// Implements srd063-timeout R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"errors"
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

// signalNames maps signal names (without SIG prefix) to their values.
// R2.1: accept signal names.
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

// errMissingOperand is a sentinel indicating insufficient positional args.
// The run function prints only the "Try --help" line for this error,
// matching GNU timeout behavior on this platform.
var errMissingOperand = errors.New("")

// config holds parsed command-line options for timeout.
type config struct {
	signal         syscall.Signal
	killAfter      time.Duration
	foreground     bool
	preserveStatus bool
	duration       time.Duration
	command        string
	cmdArgs        []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and executes the timeout command.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		if !errors.Is(err, errMissingOperand) {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		}
		printUsageError()
		return exitInternal
	}
	return executeCommand(cfg)
}

// printUsageError writes the try-help message to stderr.
func printUsageError() {
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

// parseArgs parses flags and positional arguments into a config.
func parseArgs(args []string) (*config, error) {
	cfg := &config{signal: syscall.SIGTERM}
	i, err := parseFlags(args, cfg)
	if err != nil {
		return nil, err
	}
	remaining := args[i:]
	if len(remaining) == 0 {
		return nil, errMissingOperand
	}
	dur, err := parseDuration(remaining[0])
	if err != nil {
		return nil, fmt.Errorf("invalid time interval '%s'", remaining[0])
	}
	cfg.duration = dur
	if len(remaining) < 2 {
		return nil, errMissingOperand
	}
	cfg.command = remaining[1]
	cfg.cmdArgs = remaining[2:]
	return cfg, nil
}

// parseFlags iterates through args extracting flags until a
// non-flag argument or "--" is encountered.
func parseFlags(args []string, cfg *config) (int, error) {
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			return i + 1, nil
		}
		if !strings.HasPrefix(arg, "-") || len(arg) == 1 {
			return i, nil
		}
		next, err := parseOneFlag(args, i, cfg)
		if err != nil {
			return 0, err
		}
		i = next
	}
	return i, nil
}

// parseOneFlag parses a single flag starting at args[i] and returns the
// next index to process.
func parseOneFlag(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if arg == "--foreground" {
		cfg.foreground = true
		return i + 1, nil
	}
	if arg == "--preserve-status" {
		cfg.preserveStatus = true
		return i + 1, nil
	}
	name, val, next, err := extractFlagValue(args, i)
	if err != nil {
		return 0, err
	}
	if err := applyFlagValue(name, val, cfg); err != nil {
		return 0, err
	}
	return next, nil
}

// extractFlagValue extracts the flag name and value from args[i],
// handling -sVALUE, -s VALUE, --flag=VALUE, and --flag VALUE forms.
func extractFlagValue(args []string, i int) (string, string, int, error) {
	arg := args[i]
	if strings.HasPrefix(arg, "--") {
		return extractLongFlagValue(args, i, arg)
	}
	short := string(arg[1])
	if len(arg) > 2 {
		return short, arg[2:], i + 1, nil
	}
	if i+1 >= len(args) {
		return "", "", 0, fmt.Errorf(
			"option requires an argument -- '%s'", short)
	}
	return short, args[i+1], i + 2, nil
}

// extractLongFlagValue handles --flag=VALUE and --flag VALUE forms.
func extractLongFlagValue(args []string, i int, arg string) (string, string, int, error) {
	if eqIdx := strings.IndexByte(arg, '='); eqIdx != -1 {
		return arg[2:eqIdx], arg[eqIdx+1:], i + 1, nil
	}
	name := arg[2:]
	if i+1 >= len(args) {
		return "", "", 0, fmt.Errorf(
			"option '--%s' requires an argument", name)
	}
	return name, args[i+1], i + 2, nil
}

// applyFlagValue sets the config field corresponding to the given flag name.
func applyFlagValue(name, val string, cfg *config) error {
	switch name {
	case "s", "signal":
		sig, err := parseSignal(val)
		if err != nil {
			return err
		}
		cfg.signal = sig
		return nil
	case "k", "kill-after":
		dur, err := parseDuration(val)
		if err != nil {
			return fmt.Errorf("invalid time interval '%s'", val)
		}
		cfg.killAfter = dur
		return nil
	default:
		return fmt.Errorf("unrecognized option '--%s'", name)
	}
}

// parseSignal parses a signal name or number string into a Signal value.
// R2.1: accepts names (KILL, HUP, SIGTERM) and numeric values (9, 15).
func parseSignal(s string) (syscall.Signal, error) {
	num, err := strconv.Atoi(s)
	if err == nil {
		if num < 1 {
			return 0, fmt.Errorf("invalid signal '%s'", s)
		}
		return syscall.Signal(num), nil
	}
	name := strings.TrimPrefix(strings.ToUpper(s), "SIG")
	if sig, ok := signalNames[name]; ok {
		return sig, nil
	}
	return 0, fmt.Errorf("invalid signal '%s'", s)
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
// R1.1: kill on timeout. R1.4: duration 0 means no limit.
// R2.3: --foreground skips process group creation.
func executeCommand(cfg *config) int {
	cmd := exec.Command(cfg.command, cfg.cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !cfg.foreground {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if err := cmd.Start(); err != nil {
		return handleStartError(err, cfg.command)
	}
	if cfg.duration == 0 {
		return waitForCommand(cmd)
	}
	return waitWithTimeout(cmd, cfg)
}

// waitWithTimeout waits for the command or kills it on timeout.
// Sends the configured signal when the timer fires.
func waitWithTimeout(cmd *exec.Cmd, cfg *config) int {
	done := make(chan int, 1)
	go func() {
		done <- waitForCommand(cmd)
	}()
	timer := time.NewTimer(cfg.duration)
	defer timer.Stop()
	select {
	case code := <-done:
		return code
	case <-timer.C:
		sendSignal(cmd.Process.Pid, cfg.signal, cfg.foreground)
		return handlePostTimeout(done, cmd, cfg)
	}
}

// handlePostTimeout handles the period after the initial timeout signal.
// R2.2: if kill-after is set, escalates to SIGKILL.
// R2.4: if preserve-status is set, returns the command's exit status.
func handlePostTimeout(done <-chan int, cmd *exec.Cmd, cfg *config) int {
	if cfg.killAfter > 0 {
		return waitWithKillAfter(done, cmd, cfg)
	}
	code := <-done
	if cfg.preserveStatus {
		return code
	}
	return exitTimeout
}

// waitWithKillAfter waits for the command to exit after the initial signal,
// escalating to SIGKILL if it does not exit within the kill-after duration.
// R2.2: -k DURATION sends SIGKILL after the kill-after period.
func waitWithKillAfter(done <-chan int, cmd *exec.Cmd, cfg *config) int {
	killTimer := time.NewTimer(cfg.killAfter)
	defer killTimer.Stop()
	select {
	case code := <-done:
		if cfg.preserveStatus {
			return code
		}
		return exitTimeout
	case <-killTimer.C:
		sendSignal(cmd.Process.Pid, syscall.SIGKILL, cfg.foreground)
		code := <-done
		if cfg.preserveStatus {
			return code
		}
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

// sendSignal sends the specified signal to the process or its process group.
// R2.3: in foreground mode, signals go to the process directly.
func sendSignal(pid int, sig syscall.Signal, foreground bool) {
	if foreground {
		_ = syscall.Kill(pid, sig) // best-effort signal delivery
		return
	}
	_ = syscall.Kill(-pid, sig) // best-effort signal delivery to process group
}
