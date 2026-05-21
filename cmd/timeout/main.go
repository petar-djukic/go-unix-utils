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

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "timeout: missing operand\nTry 'timeout --help' for more information.\n")
		os.Exit(125)
	}

	duration, err := parseDuration(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "timeout: invalid time interval %q\n", args[0])
		os.Exit(125)
	}

	cmdArgs := args[1:]
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		if isNotFound(err) {
			fmt.Fprintf(os.Stderr, "timeout: failed to run command %q: %v\n", cmdArgs[0], err)
			os.Exit(127)
		}
		fmt.Fprintf(os.Stderr, "timeout: failed to run command %q: %v\n", cmdArgs[0], err)
		os.Exit(126)
	}

	timedOut := false
	var timer *time.Timer
	if duration > 0 {
		timer = time.AfterFunc(duration, func() {
			timedOut = true
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		})
	}

	waitErr := cmd.Wait()
	if timer != nil {
		timer.Stop()
	}

	if waitErr == nil {
		os.Exit(0)
	}

	exitErr, ok := waitErr.(*exec.ExitError)
	if !ok {
		fmt.Fprintf(os.Stderr, "timeout: %v\n", waitErr)
		os.Exit(125)
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		os.Exit(exitErr.ExitCode())
	}

	if status.Signaled() {
		signum := int(status.Signal())
		if timedOut && status.Signal() == syscall.SIGTERM {
			os.Exit(124)
		}
		os.Exit(128 + signum)
	}

	os.Exit(exitErr.ExitCode())
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
