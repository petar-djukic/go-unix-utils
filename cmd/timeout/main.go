// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

type options struct {
	signal         syscall.Signal
	killAfter      time.Duration
	foreground     bool
	preserveStatus bool
	duration       time.Duration
	cmdArgs        []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	cmd := exec.Command(opts.cmdArgs[0], opts.cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !opts.foreground {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	if err := cmd.Start(); err != nil {
		if isNotFound(err) {
			fmt.Fprintf(os.Stderr, "timeout: failed to run command %q: %v\n", opts.cmdArgs[0], err)
			os.Exit(127)
		}
		fmt.Fprintf(os.Stderr, "timeout: failed to run command %q: %v\n", opts.cmdArgs[0], err)
		os.Exit(126)
	}

	os.Exit(runWithTimeout(cmd, opts))
}

func runWithTimeout(cmd *exec.Cmd, opts options) int {
	timedOut := false
	var timer *time.Timer
	var killTimer *time.Timer

	if opts.duration > 0 {
		timer = time.AfterFunc(opts.duration, func() {
			timedOut = true
			sendSignal(cmd, opts.signal, opts.foreground)
			if opts.signal == syscall.SIGKILL {
				_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
			}
			if opts.killAfter > 0 {
				killTimer = time.AfterFunc(opts.killAfter, func() {
					sendSignal(cmd, syscall.SIGKILL, opts.foreground)
				})
			}
		})
	}

	waitErr := cmd.Wait()
	if timer != nil {
		timer.Stop()
	}
	if killTimer != nil {
		killTimer.Stop()
	}

	if waitErr == nil {
		return 0
	}

	return exitCode(waitErr, timedOut, opts)
}

func exitCode(waitErr error, timedOut bool, opts options) int {
	exitErr, ok := waitErr.(*exec.ExitError)
	if !ok {
		fmt.Fprintf(os.Stderr, "timeout: %v\n", waitErr)
		return 125
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return exitErr.ExitCode()
	}

	if status.Signaled() {
		signum := int(status.Signal())
		if timedOut && status.Signal() == opts.signal {
			if opts.preserveStatus {
				return 128 + signum
			}
			return 124
		}
		reraiseSignal(status.Signal())
		return 128 + signum
	}

	return exitErr.ExitCode()
}

func sendSignal(cmd *exec.Cmd, sig syscall.Signal, foreground bool) {
	if foreground {
		_ = cmd.Process.Signal(sig)
	} else {
		_ = syscall.Kill(-cmd.Process.Pid, sig)
	}
}

func reraiseSignal(sig syscall.Signal) {
	syscall.Exec("/bin/sh", []string{"sh", "-c",
		fmt.Sprintf("kill -%d $$", sig)}, os.Environ())
}

func parseArgs(args []string) (options, int) {
	opts := options{signal: syscall.SIGTERM, killAfter: 0}
	i := 0

	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if !strings.HasPrefix(arg, "-") {
			break
		}
		consumed := parseLongFlag(arg, args, i, &opts)
		if consumed < 0 {
			return opts, 125
		}
		if consumed > 0 {
			i += consumed
			continue
		}
		consumed = parseShortFlags(arg, args, i, &opts)
		if consumed < 0 {
			return opts, 125
		}
		i += consumed
	}

	remaining := args[i:]
	if len(remaining) < 2 {
		fmt.Fprintf(os.Stderr, "timeout: missing operand\nTry 'timeout --help' for more information.\n")
		return opts, 125
	}

	dur, err := parseDuration(remaining[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "timeout: invalid time interval %q\n", remaining[0])
		return opts, 125
	}
	opts.duration = dur
	opts.cmdArgs = remaining[1:]
	return opts, -1
}

func parseLongFlag(arg string, args []string, i int, opts *options) int {
	switch {
	case arg == "--foreground":
		opts.foreground = true
		return 1
	case arg == "--preserve-status":
		opts.preserveStatus = true
		return 1
	case arg == "--signal":
		return parseLongValue("--signal", args, i, func(v string) int {
			return applySignal(v, opts)
		})
	case strings.HasPrefix(arg, "--signal="):
		return applySignal(arg[len("--signal="):], opts)
	case arg == "--kill-after":
		return parseLongValue("--kill-after", args, i, func(v string) int {
			return applyKillAfter(v, opts)
		})
	case strings.HasPrefix(arg, "--kill-after="):
		return applyKillAfter(arg[len("--kill-after="):], opts)
	}
	return 0
}

func parseLongValue(name string, args []string, i int, apply func(string) int) int {
	if i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "timeout: option %q requires an argument\n", name)
		return -1
	}
	result := apply(args[i+1])
	if result < 0 {
		return -1
	}
	return 2
}

func parseShortFlags(arg string, args []string, i int, opts *options) int {
	flags := arg[1:]
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 's':
			return parseShortValue('s', flags[j+1:], args, i, func(v string) int {
				return applySignal(v, opts)
			})
		case 'k':
			return parseShortValue('k', flags[j+1:], args, i, func(v string) int {
				return applyKillAfter(v, opts)
			})
		default:
			fmt.Fprintf(os.Stderr, "timeout: invalid option -- '%c'\n", flags[j])
			return -1
		}
	}
	return 1
}

func parseShortValue(flag byte, rest string, args []string, i int, apply func(string) int) int {
	if rest != "" {
		result := apply(rest)
		if result < 0 {
			return -1
		}
		return 1
	}
	if i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "timeout: option requires an argument -- '%c'\n", flag)
		return -1
	}
	result := apply(args[i+1])
	if result < 0 {
		return -1
	}
	return 2
}

func applySignal(val string, opts *options) int {
	sig, err := parseSignal(val)
	if err != nil {
		fmt.Fprintf(os.Stderr, "timeout: invalid signal %q\n", val)
		return -1
	}
	opts.signal = sig
	return 1
}

func applyKillAfter(val string, opts *options) int {
	dur, err := parseDuration(val)
	if err != nil {
		fmt.Fprintf(os.Stderr, "timeout: invalid time interval %q\n", val)
		return -1
	}
	opts.killAfter = dur
	return 1
}

func parseSignal(s string) (syscall.Signal, error) {
	s = strings.TrimPrefix(strings.ToUpper(s), "SIG")
	if num, err := strconv.Atoi(s); err == nil {
		if num < 0 {
			return 0, fmt.Errorf("invalid signal number: %d", num)
		}
		return syscall.Signal(num), nil
	}
	sig, ok := signalMap[s]
	if !ok {
		return 0, fmt.Errorf("unknown signal: %s", s)
	}
	return sig, nil
}

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

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	multiplier := time.Second
	numStr := s

	if len(s) > 0 {
		last := s[len(s)-1]
		switch last {
		case 's':
			multiplier = time.Second
			numStr = s[:len(s)-1]
		case 'm':
			multiplier = time.Minute
			numStr = s[:len(s)-1]
		case 'h':
			multiplier = time.Hour
			numStr = s[:len(s)-1]
		case 'd':
			multiplier = 24 * time.Hour
			numStr = s[:len(s)-1]
		}
	}

	if numStr == "" {
		numStr = "1"
	}

	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}

	if val < 0 {
		return 0, fmt.Errorf("negative duration")
	}

	return time.Duration(val * float64(multiplier)), nil
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), exec.ErrNotFound.Error())
}
