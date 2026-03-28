// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd055-tail follow mode (R3.1-R3.4).
// R3.1: -f/--follow polls open file descriptors for appended data.
// R3.2: --follow=name reopens by pathname on truncation/rename.
// R3.3: --pid terminates follow when the target process exits.
// R3.4: --max-unchanged-stats reopens after N unchanged poll iterations.
package main

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// followState tracks per-file state during follow mode.
type followState struct {
	name           string
	file           *os.File
	openErr        error
	size           int64
	dev            uint64
	ino            uint64
	unchangedCount int
}

// runFollow performs initial output then enters the follow polling loop.
func runFollow(cfg config, files []string, showHeader bool) int {
	states := initFollowStates(files)
	defer closeFollowStates(states)

	exitCode := outputInitial(cfg, states, showHeader)
	followLoop(cfg, states, showHeader)
	return exitCode
}

// outputInitial prints the tail output for each file before entering follow.
func outputInitial(cfg config, states []*followState, showHeader bool) int {
	exitCode := 0
	needSep := false

	for _, st := range states {
		if st.file == nil {
			fmt.Fprintf(os.Stderr, "tail: %v\n", st.openErr)
			exitCode = 1
			continue
		}
		if showHeader {
			printHeader(fileDisplayName(st.name), needSep)
		}
		if err := outputContent(st.file, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "tail: error reading '%s': %v\n", st.name, err)
			exitCode = 1
			continue
		}
		needSep = true
		recordFileInfo(st)
	}

	return exitCode
}

// followLoop polls all files for new data until the target PID exits.
// R3.1: configurable sleep interval between polls.
// R3.3: terminates when --pid process is no longer alive.
func followLoop(cfg config, states []*followState, showHeader bool) {
	interval := followInterval(cfg)
	lastPrinted := -1

	for {
		if cfg.pid > 0 && !processAlive(cfg.pid) {
			return
		}
		for i := range states {
			pollOne(states[i], cfg, showHeader, i, &lastPrinted)
		}
		time.Sleep(interval)
	}
}

// followInterval returns the polling duration from config.
func followInterval(cfg config) time.Duration {
	s := cfg.sleepInterval
	if s <= 0 {
		s = 1.0
	}
	return time.Duration(s * float64(time.Second))
}

// initFollowStates opens all files and captures initial state.
func initFollowStates(files []string) []*followState {
	states := make([]*followState, len(files))
	for i, name := range files {
		st := &followState{name: name}
		if name == "-" {
			st.file = os.Stdin
		} else if f, err := os.Open(name); err != nil {
			st.openErr = formatOpenError(name, err)
		} else {
			st.file = f
		}
		if st.file != nil {
			recordFileInfo(st)
		}
		states[i] = st
	}
	return states
}

// closeFollowStates closes all open file descriptors.
func closeFollowStates(states []*followState) {
	for _, st := range states {
		if st.file != nil && st.file != os.Stdin {
			st.file.Close()
		}
	}
}

// recordFileInfo captures size and inode from the open fd.
func recordFileInfo(st *followState) {
	if st.file == nil {
		return
	}
	info, err := st.file.Stat()
	if err != nil {
		return
	}
	st.size = info.Size()
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		st.dev = uint64(stat.Dev)
		st.ino = stat.Ino
	}
}

// pollOne checks one file for new data, dispatching by follow mode.
func pollOne(
	st *followState, cfg config, showHeader bool, idx int, lastPrinted *int,
) {
	if cfg.follow == followName {
		handleNameFollow(st, cfg)
	}
	if st.file == nil {
		if cfg.retry {
			tryReopen(st)
		}
		return
	}
	readNewData(st, showHeader, idx, lastPrinted)
}

// readNewData reads and outputs any new bytes appended to the file.
func readNewData(
	st *followState, showHeader bool, idx int, lastPrinted *int,
) {
	buf := make([]byte, 8192)
	n, _ := st.file.Read(buf)
	if n <= 0 {
		return
	}
	if showHeader && *lastPrinted != idx {
		printHeader(fileDisplayName(st.name), *lastPrinted >= 0)
	}
	*lastPrinted = idx
	os.Stdout.Write(buf[:n])
	st.size += int64(n)
	st.unchangedCount = 0
}

// handleNameFollow detects truncation and rename for --follow=name.
// R3.2: reopen on truncation or rename.
// R3.4: reopen after --max-unchanged-stats iterations.
func handleNameFollow(st *followState, cfg config) {
	if st.name == "-" {
		return
	}
	info, err := os.Stat(st.name)
	if err != nil {
		return
	}
	if detectTruncation(st, info) {
		reopenTruncated(st)
		return
	}
	if detectRename(st, info) {
		reopenByName(st)
		return
	}
	checkUnchangedStats(st, cfg, info)
}

// detectTruncation returns true when the file has shrunk.
// R3.2: size decrease indicates truncation.
func detectTruncation(st *followState, info os.FileInfo) bool {
	return info.Size() < st.size
}

// detectRename returns true when the file's identity has changed.
func detectRename(st *followState, info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return uint64(stat.Dev) != st.dev || stat.Ino != st.ino
}

// checkUnchangedStats increments the counter and reopens if threshold met.
// R3.4: reopen after maxUnchangedStats iterations with no change.
func checkUnchangedStats(st *followState, cfg config, info os.FileInfo) {
	if info.Size() == st.size {
		st.unchangedCount++
	} else {
		st.unchangedCount = 0
	}
	if cfg.maxUnchangedStats > 0 && st.unchangedCount >= cfg.maxUnchangedStats {
		reopenByName(st)
		st.unchangedCount = 0
	}
}

// reopenTruncated closes and reopens the file from the beginning.
func reopenTruncated(st *followState) {
	fmt.Fprintf(os.Stderr, "tail: %s: file truncated\n", st.name)
	closeOldFile(st)
	f, err := os.Open(st.name)
	if err != nil {
		st.file = nil
		return
	}
	st.file = f
	st.size = 0
	recordFileInfo(st)
}

// reopenByName closes and reopens the file by pathname.
func reopenByName(st *followState) {
	closeOldFile(st)
	f, err := os.Open(st.name)
	if err != nil {
		st.file = nil
		return
	}
	st.file = f
	recordFileInfo(st)
}

// closeOldFile closes the current file descriptor if open.
func closeOldFile(st *followState) {
	if st.file != nil && st.file != os.Stdin {
		st.file.Close()
		st.file = nil
	}
}

// tryReopen attempts to open a previously unavailable file.
func tryReopen(st *followState) {
	if st.name == "-" {
		return
	}
	f, err := os.Open(st.name)
	if err != nil {
		return
	}
	st.file = f
	recordFileInfo(st)
	fmt.Fprintf(os.Stderr, "tail: '%s' has appeared; following new file\n", st.name)
}

// processAlive checks if a process with the given PID exists.
// R3.3: uses signal 0 to test process liveness.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
