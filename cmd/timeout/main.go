// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/timeout implements GNU timeout: run a command with a time limit.
// Implements prd063-timeout R1.1-R1.4, R2.1-R2.4, R3.1-R3.4.
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

const (
	progName     = "timeout"
	exitTimeout  = 124
	exitInternal = 125
	exitNoExec   = 126
	exitNotFound = 127
)

// config holds parsed command-line options.
type config struct {
	signal         syscall.Signal
	killAfter      time.Duration
	hasKillAfter   bool
	foreground     bool
	preserveStatus bool
	zeroDuration   bool
	duration       time.Duration
	command        string
	args           []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		exitWithError(err.Error())
	}
	os.Exit(runTimeout(cfg))
}

// parseArgs parses command-line arguments into a config.
// R1.1: DURATION COMMAND [ARG...] with optional flags.
func parseArgs(args []string) (*config, error) {
	cfg := &config{signal: syscall.SIGTERM}
	i := 0
	for i < len(args) {
		if args[i] == "--" {
			i++
			break
		}
		if !strings.HasPrefix(args[i], "-") {
			break
		}
		consumed, err := parseFlag(cfg, args[i:])
		if err != nil {
			return nil, err
		}
		i += consumed
	}
	return finishParse(cfg, args[i:])
}

// parseFlag parses a single flag starting at args[0].
// Returns the number of args consumed.
func parseFlag(cfg *config, args []string) (int, error) {
	arg := args[0]
	switch {
	case arg == "--help":
		printHelp()
		os.Exit(0)
	case arg == "--version":
		printVersion()
		os.Exit(0)
	case arg == "--foreground":
		cfg.foreground = true
		return 1, nil
	case arg == "--preserve-status":
		cfg.preserveStatus = true
		return 1, nil
	case strings.HasPrefix(arg, "--signal="):
		return parseFlagSignalValue(cfg, arg[len("--signal="):])
	case strings.HasPrefix(arg, "--kill-after="):
		return parseFlagKillAfterValue(cfg, arg[len("--kill-after="):])
	case arg == "-s" || arg == "--signal":
		return parseFlagSignalNext(cfg, args)
	case arg == "-k" || arg == "--kill-after":
		return parseFlagKillAfterNext(cfg, args)
	case strings.HasPrefix(arg, "-s"):
		return parseFlagSignalValue(cfg, arg[2:])
	case strings.HasPrefix(arg, "-k"):
		return parseFlagKillAfterValue(cfg, arg[2:])
	}
	// TODO: --verbose not implemented per prd063-timeout non_goals (E6).
	return 0, fmt.Errorf("unrecognized option '%s'", arg)
}

// parseFlagSignalValue parses an inline signal value (e.g., --signal=KILL).
func parseFlagSignalValue(cfg *config, val string) (int, error) {
	sig, err := parseSignal(val)
	if err != nil {
		return 0, err
	}
	cfg.signal = sig
	return 1, nil
}

// parseFlagKillAfterValue parses an inline kill-after value.
func parseFlagKillAfterValue(cfg *config, val string) (int, error) {
	d, err := parseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("invalid time interval '%s'", val)
	}
	cfg.killAfter = d
	cfg.hasKillAfter = true
	return 1, nil
}

// parseFlagSignalNext parses -s SIGNAL where the value is the next arg.
func parseFlagSignalNext(cfg *config, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("option requires an argument -- 's'")
	}
	sig, err := parseSignal(args[1])
	if err != nil {
		return 0, err
	}
	cfg.signal = sig
	return 2, nil
}

// parseFlagKillAfterNext parses -k DURATION where the value is the next arg.
func parseFlagKillAfterNext(cfg *config, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("option requires an argument -- 'k'")
	}
	d, err := parseDuration(args[1])
	if err != nil {
		return 0, fmt.Errorf("invalid time interval '%s'", args[1])
	}
	cfg.killAfter = d
	cfg.hasKillAfter = true
	return 2, nil
}

// finishParse extracts DURATION and COMMAND from remaining positional args.
func finishParse(cfg *config, rest []string) (*config, error) {
	if len(rest) == 0 {
		return nil, fmt.Errorf("missing operand")
	}
	if len(rest) < 2 {
		return nil, fmt.Errorf("missing operand after '%s'", rest[0])
	}
	d, err := parseDuration(rest[0])
	if err != nil {
		return nil, fmt.Errorf("invalid time interval '%s'", rest[0])
	}
	cfg.duration = d
	cfg.zeroDuration = isDurationZero(rest[0])
	cfg.command = rest[1]
	cfg.args = rest[2:]
	return cfg, nil
}

// isDurationZero checks if a duration string represents zero.
// R1.4: duration 0 disables the timeout.
func isDurationZero(s string) bool {
	numStr := s
	last := s[len(s)-1]
	if _, ok := suffixMultiplier(last); ok {
		numStr = s[:len(s)-1]
	}
	v, err := strconv.ParseFloat(numStr, 64)
	return err == nil && v == 0
}

// runTimeout executes the command with the configured timeout.
// R1.1: launch command, start timer, send signal on expiry.
// R3.1: create a new process group unless --foreground is set.
// R3.2: forward signals to the child process or process group.
func runTimeout(cfg *config) int {
	cmd := exec.Command(cfg.command, cfg.args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !cfg.foreground {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if err := cmd.Start(); err != nil {
		return handleStartError(err)
	}
	setupSignalForwarding(cmd, cfg.foreground)
	if cfg.zeroDuration {
		return waitForExit(cmd)
	}
	return waitWithTimeout(cmd, cfg)
}

// setupSignalForwarding registers to receive common signals and forwards
// them to the child process or process group.
// R3.2: signals received by timeout are forwarded to the child.
func setupSignalForwarding(cmd *exec.Cmd, foreground bool) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh,
		syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT,
		syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)
	go forwardSignals(sigCh, cmd, foreground)
}

// forwardSignals reads signals from the channel and sends them to the child.
func forwardSignals(sigCh <-chan os.Signal, cmd *exec.Cmd, foreground bool) {
	for sig := range sigCh {
		if s, ok := sig.(syscall.Signal); ok {
			sendSignal(cmd, s, foreground)
		}
	}
}

// handleStartError returns the appropriate exit code for a launch failure.
// R3.4: command not found exits 127, other exec failures exit 126.
func handleStartError(err error) int {
	fmt.Fprintf(os.Stderr, "%s: failed to run command: %s\n", progName, err)
	if isCommandNotFound(err) {
		return exitNotFound
	}
	return exitNoExec
}

// isCommandNotFound checks if the error indicates the command was not found.
func isCommandNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "no such file")
}

// waitForExit waits for the command to complete with no timeout.
// R1.4: duration 0 means no time limit.
// R3.1: exit with the command's exit status.
func waitForExit(cmd *exec.Cmd) int {
	if err := cmd.Wait(); err != nil {
		return exitCodeFromError(err)
	}
	return 0
}

// waitWithTimeout waits for the command with a timer.
// R1.1: sends the configured signal when the timer expires.
func waitWithTimeout(cmd *exec.Cmd, cfg *config) int {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(cfg.duration)
	defer timer.Stop()

	select {
	case err := <-done:
		if err == nil {
			return 0
		}
		return exitCodeFromError(err)
	case <-timer.C:
		return handleExpiry(cmd, cfg, done)
	}
}

// handleExpiry sends the configured signal and waits for exit.
// R1.1: send signal on timeout.
// R2.2: optionally escalate to SIGKILL.
// R2.4: use preserve-status exit code when configured.
func handleExpiry(cmd *exec.Cmd, cfg *config, done <-chan error) int {
	sendSignal(cmd, cfg.signal, cfg.foreground)
	if !cfg.hasKillAfter {
		err := <-done
		return timeoutExitCode(cfg, err)
	}
	return waitForKillAfter(cmd, cfg, done)
}

// waitForKillAfter waits for the kill-after period, sending SIGKILL if needed.
// R2.2: send SIGKILL if the command is still running after the grace period.
func waitForKillAfter(cmd *exec.Cmd, cfg *config, done <-chan error) int {
	killTimer := time.NewTimer(cfg.killAfter)
	defer killTimer.Stop()

	select {
	case err := <-done:
		return timeoutExitCode(cfg, err)
	case <-killTimer.C:
		sendSignal(cmd, syscall.SIGKILL, cfg.foreground)
		err := <-done
		return timeoutExitCode(cfg, err)
	}
}

// timeoutExitCode returns the appropriate exit code after a timeout.
// R2.4: --preserve-status returns the command's actual exit status
// instead of the timeout-specific exit code 124.
func timeoutExitCode(cfg *config, err error) int {
	if cfg.preserveStatus {
		if err == nil {
			return 0
		}
		return exitCodeFromError(err)
	}
	return exitTimeout
}

// sendSignal sends a signal to the command's process or process group.
// R2.3: when --foreground is set, signal only the child PID.
// R3.1: otherwise, signal the entire process group.
func sendSignal(cmd *exec.Cmd, sig syscall.Signal, foreground bool) {
	if cmd.Process == nil {
		return
	}
	if foreground {
		_ = cmd.Process.Signal(sig) // best-effort: process may have exited
		return
	}
	// Send to the entire process group.
	_ = syscall.Kill(-cmd.Process.Pid, sig) // best-effort: process may have exited
}

// exitCodeFromError extracts the exit code from a command execution error.
// R3.3: if the command was killed by a signal, return 128+signum.
func exitCodeFromError(err error) int {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return exitInternal
	}
	ws, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return exitErr.ExitCode()
	}
	if ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return exitErr.ExitCode()
}

// parseSignal converts a signal name or number to a syscall.Signal.
// R2.1: accepts names (TERM, HUP, KILL) and numeric values.
func parseSignal(s string) (syscall.Signal, error) {
	if n, err := strconv.Atoi(s); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("invalid signal '%s'", s)
		}
		return syscall.Signal(n), nil
	}
	name := strings.TrimPrefix(strings.ToUpper(s), "SIG")
	sig, ok := signalMap[name]
	if !ok {
		return 0, fmt.Errorf("invalid signal '%s'", s)
	}
	return sig, nil
}

// signalMap maps POSIX signal names (without SIG prefix) to syscall.Signal.
var signalMap = map[string]syscall.Signal{
	"HUP":    syscall.SIGHUP,
	"INT":    syscall.SIGINT,
	"QUIT":   syscall.SIGQUIT,
	"ILL":    syscall.SIGILL,
	"TRAP":   syscall.SIGTRAP,
	"ABRT":   syscall.SIGABRT,
	"BUS":    syscall.SIGBUS,
	"FPE":    syscall.SIGFPE,
	"KILL":   syscall.SIGKILL,
	"USR1":   syscall.SIGUSR1,
	"SEGV":   syscall.SIGSEGV,
	"USR2":   syscall.SIGUSR2,
	"PIPE":   syscall.SIGPIPE,
	"ALRM":   syscall.SIGALRM,
	"TERM":   syscall.SIGTERM,
	"CHLD":   syscall.SIGCHLD,
	"CONT":   syscall.SIGCONT,
	"STOP":   syscall.SIGSTOP,
	"TSTP":   syscall.SIGTSTP,
	"TTIN":   syscall.SIGTTIN,
	"TTOU":   syscall.SIGTTOU,
	"URG":    syscall.SIGURG,
	"XCPU":   syscall.SIGXCPU,
	"XFSZ":   syscall.SIGXFSZ,
	"VTALRM": syscall.SIGVTALRM,
	"PROF":   syscall.SIGPROF,
	"WINCH":  syscall.SIGWINCH,
	"IO":     syscall.SIGIO,
	"SYS":    syscall.SIGSYS,
}

// parseDuration parses a GNU-style duration value.
// R1.2: fractional seconds. R1.3: suffix s/m/h/d.
func parseDuration(arg string) (time.Duration, error) {
	if arg == "" {
		return 0, fmt.Errorf("invalid time interval ''")
	}
	multiplier := time.Second
	numStr := arg
	last := arg[len(arg)-1]
	if suffix, ok := suffixMultiplier(last); ok {
		multiplier = suffix
		numStr = arg[:len(arg)-1]
	}
	seconds, err := strconv.ParseFloat(numStr, 64)
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("invalid time interval '%s'", arg)
	}
	nanos := seconds * float64(multiplier)
	return time.Duration(nanos), nil
}

// suffixMultiplier returns the duration multiplier for a suffix character.
func suffixMultiplier(c byte) (time.Duration, bool) {
	switch c {
	case 's':
		return time.Second, true
	case 'm':
		return time.Minute, true
	case 'h':
		return time.Hour, true
	case 'd':
		return 24 * time.Hour, true
	default:
		return 0, false
	}
}

// printHelp outputs usage information to stdout.
func printHelp() {
	fmt.Printf("Usage: %s [OPTION] DURATION COMMAND [ARG]...\n", progName)
	fmt.Printf("  or:  %s [OPTION]\n", progName)
	fmt.Println("Start COMMAND, and kill it if still running after DURATION.")
	fmt.Println()
	helpFlags()
}

// helpFlags prints the flag descriptions for --help output.
func helpFlags() {
	fmt.Println("      --foreground      when not running timeout directly from a shell prompt,")
	fmt.Println("                          allow COMMAND to read from the TTY and get TTY signals")
	fmt.Println("  -k, --kill-after=DURATION")
	fmt.Println("                        also send a KILL signal if COMMAND is still running")
	fmt.Println("                          this long after the initial signal was sent")
	fmt.Println("  -s, --signal=SIGNAL   specify the signal to be sent on timeout;")
	fmt.Println("                          SIGNAL may be a name like 'HUP' or a number")
	fmt.Println("      --preserve-status exit with the same status as COMMAND, even when the")
	fmt.Println("                          command times out")
	fmt.Println("      --help     display this help and exit")
	fmt.Println("      --version  output version information and exit")
	fmt.Println()
	fmt.Println("DURATION is a floating point number with an optional suffix:")
	fmt.Println("'s' for seconds (the default), 'm' for minutes, 'h' for hours or 'd' for days.")
	fmt.Println("A duration of 0 disables the associated timeout.")
}

// printVersion outputs version information to stdout.
func printVersion() {
	fmt.Printf("%s (go-unix-utils)\n", progName)
}

// exitWithError prints an error message and exits with status 125.
func exitWithError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", progName, msg)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
	os.Exit(exitInternal)
}
