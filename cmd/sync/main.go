// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/sync: synchronize cached writes to persistent storage.
// Implements srd085 R1.1-R1.4, R2.1-R2.3.
package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "sync"

type config struct {
	data       bool
	fileSystem bool
}

type runAction int

const (
	doRun runAction = iota
	doHelp
	doVersion
	doError
)

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, files, act := parseArgs(os.Args[1:])
	switch act {
	case doHelp:
		printHelp()
	case doVersion:
		printVersion()
	case doRun:
		if !run(cfg, files) {
			os.Exit(1)
		}
	case doError:
		os.Exit(1)
	}
}

// run executes the sync operation and returns true on success.
// R1.1: no files → global sync. R1.2-R1.4: per-file sync.
func run(cfg config, files []string) bool {
	// R1.3: --data requires file arguments.
	if cfg.data && len(files) == 0 {
		fmt.Fprintf(os.Stderr,
			"%s: --data needs at least one argument\n", programName)
		return false
	}

	// R1.1: no arguments — global sync(2).
	if len(files) == 0 {
		syscall.Sync()
		return true
	}

	// R1.4/R2.1: pure --file-system mode without --data. On macOS,
	// syncfs(2) is unavailable so GNU falls back to global sync(2)
	// without opening individual files.
	if cfg.fileSystem && !cfg.data {
		syncFilesystem(0)
		return true
	}

	ok := true
	for _, name := range files {
		if err := syncFile(name, cfg); err != nil {
			ok = false
		}
	}
	return ok
}

// syncFile opens a file and calls the appropriate sync operation.
func syncFile(name string, cfg config) error {
	f, err := os.Open(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: error opening '%s': %s\n",
			programName, name, unwrapErr(err))
		return err
	}
	defer f.Close() // best-effort close

	return applySyncOp(f, name, cfg)
}

// applySyncOp dispatches to the correct sync syscall based on flags.
// Matches GNU coreutils logic: syncfs and fdatasync/fsync are independent.
func applySyncOp(f *os.File, name string, cfg config) error {
	fd := int(f.Fd())
	var failed bool

	// R1.4: sync filesystem containing the file.
	if cfg.fileSystem {
		syncFilesystem(fd)
	}

	if cfg.data {
		// R1.3: fdatasync — sync data only, skip metadata.
		if err := fdatasync(fd); err != nil {
			printSyncErr(name, err)
			failed = true
		}
	} else if !cfg.fileSystem {
		// R1.2: fsync — sync data and metadata (default).
		if err := f.Sync(); err != nil {
			printSyncErr(name, err)
			failed = true
		}
	}

	if failed {
		return fmt.Errorf("sync failed")
	}
	return nil
}

// fdatasync calls fdatasync(2) to sync file data without metadata.
func fdatasync(fd int) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_FDATASYNC, uintptr(fd), 0, 0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// syncFilesystem syncs the filesystem containing the open file descriptor.
// On macOS, syncfs(2) is not available; falls back to sync(2).
func syncFilesystem(_ int) {
	syscall.Sync()
}

// printSyncErr prints a sync error message matching GNU format.
func printSyncErr(name string, err error) {
	fmt.Fprintf(os.Stderr, "%s: error syncing '%s': %s\n",
		programName, name, err.Error())
}

// unwrapErr extracts the underlying error message from os.PathError.
func unwrapErr(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printTryHelp prints the standard "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr,
		"Try '%s --help' for more information.\n", programName)
}

// parseArgs parses command-line arguments matching GNU sync behavior.
func parseArgs(args []string) (config, []string, runAction) {
	var cfg config
	var files []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone || !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if act := parseLongFlag(arg[2:], &cfg); act != doRun {
				return cfg, files, act
			}
			continue
		}
		if act := parseShortFlags(arg[1:], &cfg); act != doRun {
			return cfg, files, act
		}
	}
	return cfg, files, doRun
}

// parseLongFlag handles a single --flag.
func parseLongFlag(flag string, cfg *config) runAction {
	switch flag {
	case "data":
		cfg.data = true
	case "file-system":
		cfg.fileSystem = true
	case "help":
		return doHelp
	case "version":
		return doVersion
	default:
		fmt.Fprintf(os.Stderr,
			"%s: unrecognized option '--%s'\n", programName, flag)
		printTryHelp()
		return doError
	}
	return doRun
}

// parseShortFlags handles combined short flags like -df.
func parseShortFlags(flags string, cfg *config) runAction {
	for _, ch := range flags {
		switch ch {
		case 'd':
			cfg.data = true
		case 'f':
			cfg.fileSystem = true
		default:
			fmt.Fprintf(os.Stderr,
				"%s: invalid option -- '%c'\n", programName, ch)
			printTryHelp()
			return doError
		}
	}
	return doRun
}

func printHelp() {
	fmt.Print(`Usage: sync [OPTION] [FILE]...
Synchronize cached writes to persistent storage.

If one or more files are specified, sync only them,
or their containing file systems.

  -d, --data             sync only file data, no unneeded metadata
  -f, --file-system      sync the file systems that contain the files
      --help             display this help and exit
      --version          output version information and exit
`)
}

func printVersion() {
	fmt.Println("sync (go-unix-utils)")
}
