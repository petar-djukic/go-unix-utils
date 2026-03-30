// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/touch implements GNU touch: create files and update timestamps.
//
// Implements prd062-touch R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// touchOptions holds parsed flag state.
type touchOptions struct {
	noCreate   bool   // -c, --no-create: do not create files
	accessOnly bool   // R2.1: -a changes only access time
	modOnly    bool   // R2.2: -m changes only modification time
	stamp      string // R2.4: -t [[CC]YY]MMDDhhmm[.ss]
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run parses flags and processes each file argument.
// R1.4: processes multiple files in order.
// R4.1: exits 0 on success, 1 on any error.
func run(args []string, stderr *os.File) int {
	opts, files := parseArgs(args)

	if len(files) == 0 {
		fmt.Fprintln(stderr, "touch: missing file operand")
		return 1
	}

	now := time.Now()
	t, err := resolveTimestamp(opts, now)
	if err != nil {
		fmt.Fprintf(stderr, "touch: %v\n", err)
		return 1
	}

	exitCode := 0
	for _, f := range files {
		if err := touchFile(f, t, opts); err != nil {
			fmt.Fprintf(stderr, "touch: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// resolveTimestamp determines the timestamp to apply.
// R2.4: -t STAMP overrides current time.
func resolveTimestamp(opts touchOptions, now time.Time) (time.Time, error) {
	if opts.stamp != "" {
		return parseStamp(opts.stamp, now)
	}
	return now, nil
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (touchOptions, []string) {
	var opts touchOptions
	var files []string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "--no-create" {
			opts.noCreate = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			// Unknown long flag — treat as file
			files = append(files, arg)
			continue
		}
		if len(arg) >= 2 && arg[0] == '-' {
			needsArg := parseShortFlags(&opts, arg[1:])
			if needsArg && i+1 < len(args) {
				i++
				opts.stamp = args[i]
			}
			continue
		}
		files = append(files, arg)
	}
	return opts, files
}

// parseShortFlags processes short flag characters.
// Returns true when -t appears at the end and needs the next argument.
func parseShortFlags(opts *touchOptions, chars string) bool {
	for i, ch := range chars {
		switch ch {
		case 'c':
			opts.noCreate = true
		case 'a':
			// R2.1: change only access time
			opts.accessOnly = true
		case 'm':
			// R2.2: change only modification time
			opts.modOnly = true
		case 't':
			// R2.4: -t takes the rest of the cluster or next arg
			rest := chars[i+1:]
			if rest != "" {
				opts.stamp = rest
				return false
			}
			return true
		}
	}
	return false
}

// parseStamp parses the -t [[CC]YY]MMDDhhmm[.ss] format.
func parseStamp(stamp string, now time.Time) (time.Time, error) {
	base, sec, err := splitStampSeconds(stamp)
	if err != nil {
		return time.Time{}, err
	}

	year, month, day, hour, min, err := parseStampFields(base, now)
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local), nil
}

// splitStampSeconds separates the optional .ss seconds from the stamp.
func splitStampSeconds(stamp string) (string, int, error) {
	dot := strings.LastIndex(stamp, ".")
	if dot < 0 {
		return stamp, 0, nil
	}
	secStr := stamp[dot+1:]
	sec, err := strconv.Atoi(secStr)
	if err != nil || sec < 0 || sec > 61 {
		return "", 0, fmt.Errorf("invalid date format %q", stamp)
	}
	return stamp[:dot], sec, nil
}

// parseStampFields extracts year, month, day, hour, minute from the base.
func parseStampFields(base string, now time.Time) (int, int, int, int, int, error) {
	switch len(base) {
	case 8:
		return parseMMDDhhmm(base, now.Year())
	case 10:
		return parseYYMMDDhhmm(base)
	case 12:
		return parseCCYYMMDDhhmm(base)
	default:
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format %q", base)
	}
}

// parseMMDDhhmm parses 8-char stamp using the default year.
func parseMMDDhhmm(s string, year int) (int, int, int, int, int, error) {
	month, day, hour, min, err := parseDateTimeDigits(s)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	return year, month, day, hour, min, nil
}

// parseYYMMDDhhmm parses 10-char stamp with 2-digit year.
func parseYYMMDDhhmm(s string) (int, int, int, int, int, error) {
	yy, err := strconv.Atoi(s[:2])
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format %q", s)
	}
	month, day, hour, min, err := parseDateTimeDigits(s[2:])
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	return expandTwoDigitYear(yy), month, day, hour, min, nil
}

// parseCCYYMMDDhhmm parses 12-char stamp with 4-digit year.
func parseCCYYMMDDhhmm(s string) (int, int, int, int, int, error) {
	year, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format %q", s)
	}
	month, day, hour, min, err := parseDateTimeDigits(s[4:])
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	return year, month, day, hour, min, nil
}

// parseDateTimeDigits parses exactly 8 digits as MMDDhhmm.
func parseDateTimeDigits(s string) (int, int, int, int, error) {
	if len(s) != 8 {
		return 0, 0, 0, 0, fmt.Errorf("invalid date format %q", s)
	}
	month, err := strconv.Atoi(s[0:2])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid date format %q", s)
	}
	day, err := strconv.Atoi(s[2:4])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid date format %q", s)
	}
	hour, err := strconv.Atoi(s[4:6])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid date format %q", s)
	}
	min, err := strconv.Atoi(s[6:8])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid date format %q", s)
	}
	return month, day, hour, min, nil
}

// expandTwoDigitYear converts YY to CCYY per GNU convention.
// 69-99 → 1969-1999, 00-68 → 2000-2068.
func expandTwoDigitYear(yy int) int {
	if yy >= 69 {
		return 1900 + yy
	}
	return 2000 + yy
}

// touchFile updates timestamps or creates a single file.
// R1.1: updates atime and mtime to t.
// R1.2: creates the file if it does not exist.
// R1.3: skips creation when noCreate is set.
func touchFile(path string, t time.Time, opts touchOptions) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if opts.noCreate {
			return nil
		}
		return createAndTouch(path, t, opts)
	}
	if err != nil {
		return err
	}
	return applyTimestamps(path, t, opts)
}

// createAndTouch creates an empty file and sets its timestamps.
func createAndTouch(path string, t time.Time, opts touchOptions) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	f.Close() // best-effort; error on applyTimestamps is more important
	return applyTimestamps(path, t, opts)
}

// applyTimestamps sets atime and/or mtime based on -a/-m flags.
// R2.1: -a alone changes only access time, preserving mtime.
// R2.2: -m alone changes only modification time, preserving atime.
// R2.3: neither -a nor -m (or both) changes both times.
func applyTimestamps(path string, t time.Time, opts touchOptions) error {
	atime, mtime := t, t
	if opts.accessOnly != opts.modOnly {
		fi, err := sys.Stat(path)
		if err != nil {
			return err
		}
		if opts.accessOnly {
			mtime = fi.ModTime
		} else {
			atime = fi.AccessTime
		}
	}
	return os.Chtimes(path, atime, mtime)
}
