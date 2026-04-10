// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/touch: create files and update timestamps.
// Implements srd062-touch R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "touch"

const tryHelp = "Try 'touch --help' for more information."

// helpText is the usage message printed for --help.
const helpText = `Usage: touch [OPTION]... FILE...
Update the access and modification times of each FILE to the current time.

A FILE argument that does not exist is created empty, unless -c or -h
is supplied.

      -a                     change only the access time
      -c, --no-create        do not create any files
      -d, --date=STRING      parse STRING and use it instead of current time
      -h, --no-dereference   affect each symbolic link instead of any referenced
                             file (useful only on systems that can change the
                             timestamps of a symlink)
      -m                     change only the modification time
      -r, --reference=FILE   use this file's times instead of current time
      -t STAMP               use [[CC]YY]MMDDhhmm[.ss] instead of current time
      --help        display this help and exit
      --version     output version information and exit
`

// versionText is the version string printed for --version.
const versionText = "touch (go-unix-utils) 1.0\n"

// options holds parsed command-line flags.
type options struct {
	noCreate   bool
	accessOnly bool   // R2.1: -a flag
	modOnly    bool   // R2.2: -m flag
	stamp      string // R2.4: -t STAMP value
	files      []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the touch logic and returns the exit code.
// R1.1: update access and modification times to current time.
// R1.2: create file if absent. R1.3: -c suppresses creation.
// R1.4: process multiple file arguments in order.
// R2.1-R2.4: timestamp selection and explicit times.
func run(args []string) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n%s\n", progName, err, tryHelp)
		return 1
	}
	if opts == nil {
		return 0 // --help or --version handled
	}
	if len(opts.files) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\n%s\n", progName, tryHelp)
		return 1
	}
	ts, err := resolveTime(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	exitCode := 0
	for _, f := range opts.files {
		if err := touchFile(f, opts, ts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// resolveTime determines the timestamp to apply.
// R2.3: default is current time. R2.4: -t overrides with parsed stamp.
func resolveTime(opts *options) (time.Time, error) {
	if opts.stamp != "" {
		return parseStamp(opts.stamp)
	}
	return time.Now(), nil
}

// parseArgs parses command-line arguments into options.
// Returns nil options when --help or --version was handled.
func parseArgs(args []string) (*options, error) {
	opts := &options{}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			opts.files = append(opts.files, args[i:]...)
			return opts, nil
		}
		if arg == "--help" {
			fmt.Fprint(os.Stdout, helpText)
			return nil, nil
		}
		if arg == "--version" {
			fmt.Fprint(os.Stdout, versionText)
			return nil, nil
		}
		if arg == "--no-create" {
			opts.noCreate = true
			i++
			continue
		}
		if handled, advance, err := parseLongWithValue(arg, args, i); handled {
			if err != nil {
				return nil, err
			}
			i += advance
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			advance, err := parseShortFlags(arg[1:], args, i, opts)
			if err != nil {
				return nil, err
			}
			i += advance
			continue
		}
		opts.files = append(opts.files, arg)
		i++
	}
	return opts, nil
}

// parseLongWithValue handles --date=X and --reference=X flags.
// Returns (handled, advance, error). These are R3 flags; we
// accept them for parsing but they are no-ops until R3 scope.
func parseLongWithValue(arg string, args []string, idx int) (bool, int, error) {
	if strings.HasPrefix(arg, "--date=") || strings.HasPrefix(arg, "--reference=") {
		return true, 1, nil
	}
	if arg == "--date" || arg == "--reference" {
		if idx+1 >= len(args) {
			return true, 1, fmt.Errorf("option '%s' requires an argument", arg)
		}
		return true, 2, nil
	}
	return false, 0, nil
}

// parseShortFlags processes a cluster of short flags (e.g., "-acm").
// R1.3: -c sets noCreate. R2.1: -a sets accessOnly. R2.2: -m sets modOnly.
// R2.4: -t captures the stamp value.
func parseShortFlags(flags string, args []string, idx int, opts *options) (int, error) {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'a':
			opts.accessOnly = true
		case 'm':
			opts.modOnly = true
		case 'c':
			opts.noCreate = true
		case 'h':
			// R3.4: --no-dereference; accepted for parsing, no-op until R3 scope.
		case 't':
			return consumeValueFlag(flags, args, idx, j, 't', &opts.stamp)
		case 'r':
			// -r FILE: consume the value argument (R3 scope, no-op).
			var discard string
			return consumeValueFlag(flags, args, idx, j, 'r', &discard)
		case 'd':
			// -d STRING: consume the value argument (R3 scope, no-op).
			var discard string
			return consumeValueFlag(flags, args, idx, j, 'd', &discard)
		default:
			return 1, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return 1, nil
}

// consumeValueFlag extracts the value for a flag that takes an argument.
// If characters remain in the cluster after the flag, they are the value.
// Otherwise the next argument is consumed.
func consumeValueFlag(flags string, args []string, idx, j int, flag byte, dest *string) (int, error) {
	if j+1 < len(flags) {
		*dest = flags[j+1:]
		return 1, nil
	}
	if idx+1 >= len(args) {
		return 1, fmt.Errorf("option requires an argument -- '%c'", flag)
	}
	*dest = args[idx+1]
	return 2, nil
}

// touchFile updates timestamps or creates a file.
// R1.1: update times. R1.2: create file if absent.
// R1.3: skip creation when noCreate is true.
func touchFile(path string, opts *options, ts time.Time) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if opts.noCreate {
			return nil // R1.3: suppress creation silently
		}
		if err := createEmpty(path); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("cannot touch '%s': %v", path, err)
	}
	return applyTimestamps(path, opts, ts)
}

// createEmpty creates an empty file with default permissions.
// R1.2: create file as empty with default permissions.
func createEmpty(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot touch '%s': %v", path, err)
	}
	f.Close() // best-effort close; file is empty
	return nil
}

// applyTimestamps sets atime and/or mtime based on -a/-m flags.
// R2.1: -a changes only access time.
// R2.2: -m changes only modification time.
// R2.3: neither -a nor -m changes both.
func applyTimestamps(path string, opts *options, ts time.Time) error {
	fi, err := sys.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot touch '%s': %v", path, err)
	}
	atime := fi.AccessTime
	mtime := fi.ModTime
	changeAccess := !opts.modOnly || opts.accessOnly
	changeMod := !opts.accessOnly || opts.modOnly
	if changeAccess {
		atime = ts
	}
	if changeMod {
		mtime = ts
	}
	if err := os.Chtimes(path, atime, mtime); err != nil {
		return fmt.Errorf("cannot touch '%s': %v", path, err)
	}
	return nil
}

// parseStamp parses [[CC]YY]MMDDhhmm[.ss] into a time.Time.
// R2.4: -t STAMP timestamp format.
func parseStamp(stamp string) (time.Time, error) {
	base, sec, err := splitStampSeconds(stamp)
	if err != nil {
		return time.Time{}, err
	}
	year, month, day, hour, min, err := parseStampFields(base, stamp)
	if err != nil {
		return time.Time{}, err
	}
	t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local)
	if t.Month() != time.Month(month) || t.Day() != day {
		return time.Time{}, fmt.Errorf("invalid date format '%s'", stamp)
	}
	return t, nil
}

// splitStampSeconds separates the optional .ss suffix from a stamp string.
func splitStampSeconds(stamp string) (string, int, error) {
	base, secStr, hasDot := strings.Cut(stamp, ".")
	if !hasDot {
		return stamp, 0, nil
	}
	if len(secStr) != 2 {
		return "", 0, fmt.Errorf("invalid date format '%s'", stamp)
	}
	sec, err := strconv.Atoi(secStr)
	if err != nil || sec < 0 || sec > 61 {
		return "", 0, fmt.Errorf("invalid date format '%s'", stamp)
	}
	return base, sec, nil
}

// parseStampFields extracts year, month, day, hour, min from the base
// portion of a stamp (without the .ss suffix).
func parseStampFields(base, stamp string) (int, int, int, int, int, error) {
	var year, month, day, hour, min int
	var err error
	switch len(base) {
	case 8: // MMDDhhmm — use current year
		year = time.Now().Year()
		month, day, hour, min, err = parseDateFields(base)
	case 10: // YYMMDDhhmm — two-digit year
		year, err = parseTwoDigitYear(base[:2], stamp)
		if err != nil {
			return 0, 0, 0, 0, 0, err
		}
		month, day, hour, min, err = parseDateFields(base[2:])
	case 12: // CCYYMMDDhhmm — four-digit year
		year, err = strconv.Atoi(base[:4])
		if err != nil {
			return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format '%s'", stamp)
		}
		month, day, hour, min, err = parseDateFields(base[4:])
	default:
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format '%s'", stamp)
	}
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format '%s'", stamp)
	}
	return year, month, day, hour, min, nil
}

// parseTwoDigitYear converts a two-digit year string to a four-digit year.
// 69-99 maps to 1969-1999; 00-68 maps to 2000-2068.
func parseTwoDigitYear(s, stamp string) (int, error) {
	yy, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid date format '%s'", stamp)
	}
	if yy >= 69 {
		return 1900 + yy, nil
	}
	return 2000 + yy, nil
}

// parseDateFields extracts month, day, hour, minute from an 8-character
// string in MMDDhhmm format.
func parseDateFields(s string) (int, int, int, int, error) {
	if len(s) != 8 {
		return 0, 0, 0, 0, fmt.Errorf("invalid length")
	}
	month, err := strconv.Atoi(s[0:2])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	day, err := strconv.Atoi(s[2:4])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	hour, err := strconv.Atoi(s[4:6])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	min, err := strconv.Atoi(s[6:8])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return month, day, hour, min, nil
}
