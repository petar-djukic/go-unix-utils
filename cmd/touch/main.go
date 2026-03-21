// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd062-touch R1.1–R1.4: default file creation,
// timestamp update, multiple file arguments, and -c/--no-create.
// Implements prd062-touch R2.1–R2.4: -a (access only), -m (modification only),
// -t STAMP (explicit timestamp).
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "touch"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr, time.Now()))
}

// touchOpts holds parsed command-line options for the touch utility.
type touchOpts struct {
	noCreate   bool
	accessOnly bool       // R2.1: -a flag
	modOnly    bool       // R2.2: -m flag
	timestamp  *time.Time // R2.4: -t STAMP
	files      []string
}

// changeBoth returns true when both access and modification times
// should be changed. R2.3: when neither -a nor -m is given, change both.
// Also change both when both -a and -m are given.
func (o touchOpts) changeBoth() bool {
	return (!o.accessOnly && !o.modOnly) || (o.accessOnly && o.modOnly)
}

// effectiveTime returns the timestamp to use for the touch operation.
func (o touchOpts) effectiveTime(now time.Time) time.Time {
	if o.timestamp != nil {
		return *o.timestamp
	}
	return now
}

// run parses arguments and processes each file.
func run(args []string, stderr io.Writer, now time.Time) int {
	opts, code := parseArgs(args, stderr)
	if code >= 0 {
		return code
	}
	exitCode := 0
	t := opts.effectiveTime(now)
	for _, file := range opts.files {
		if err := touchFile(file, t, opts, stderr); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// touchFile creates or updates timestamps for a single file.
func touchFile(path string, t time.Time, opts touchOpts, stderr io.Writer) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return handleMissing(path, t, opts, stderr)
	}
	if err != nil {
		reportError(path, stripPathError(err), stderr)
		return err
	}
	return applyTimestamps(path, t, opts, stderr)
}

// handleMissing creates the file or skips it based on noCreate.
// R1.2: create empty file. R1.3: suppress with -c.
func handleMissing(path string, t time.Time, opts touchOpts, stderr io.Writer) error {
	if opts.noCreate {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		reportError(path, stripPathError(err), stderr)
		return err
	}
	f.Close() // best-effort close, file created successfully
	return applyTimestamps(path, t, opts, stderr)
}

// applyTimestamps sets access and/or modification times based on opts.
// R2.1: -a changes only access time.
// R2.2: -m changes only modification time.
// R2.3: neither -a nor -m changes both.
func applyTimestamps(path string, t time.Time, opts touchOpts, stderr io.Writer) error {
	atime, mtime := t, t
	if !opts.changeBoth() {
		cur, err := readCurrentTimes(path)
		if err != nil {
			reportError(path, stripPathError(err), stderr)
			return err
		}
		if opts.accessOnly {
			mtime = cur.mtime
		} else {
			atime = cur.atime
		}
	}
	if err := os.Chtimes(path, atime, mtime); err != nil {
		reportError(path, stripPathError(err), stderr)
		return err
	}
	return nil
}

// fileTimes holds the access and modification times of a file.
type fileTimes struct {
	atime time.Time
	mtime time.Time
}

// readCurrentTimes reads the current access and modification times.
func readCurrentTimes(path string) (fileTimes, error) {
	fi, err := sys.Stat(path)
	if err != nil {
		return fileTimes{}, err
	}
	return fileTimes{atime: fi.AccessTime, mtime: fi.ModTime}, nil
}

// reportError prints a standardized error message to stderr.
func reportError(path, msg string, stderr io.Writer) {
	fmt.Fprintf(stderr, "%s: cannot touch '%s': %s\n", progName, path, msg)
}

// parseArgs extracts options from args.
// Returns (opts, exitCode); exitCode -1 means continue.
func parseArgs(args []string, stderr io.Writer) (touchOpts, int) {
	var opts touchOpts
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			opts.files = append(opts.files, args[i+1:]...)
			return finishParse(opts, stderr)
		case arg == "--no-create":
			opts.noCreate = true
		case arg == "--help":
			printHelp(os.Stdout)
			return touchOpts{}, 0
		case arg == "--version":
			printVersion(os.Stdout)
			return touchOpts{}, 0
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
			printTryHelp(stderr)
			return touchOpts{}, 1
		case len(arg) > 1 && arg[0] == '-':
			code := parseShortFlags(arg[1:], args, &i, &opts, stderr)
			if code >= 0 {
				return touchOpts{}, code
			}
		default:
			opts.files = append(opts.files, arg)
		}
	}
	return finishParse(opts, stderr)
}

// parseShortFlags processes a string of single-character flags.
func parseShortFlags(flags string, args []string, idx *int, opts *touchOpts, stderr io.Writer) int {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'a':
			opts.accessOnly = true
		case 'm':
			opts.modOnly = true
		case 'c':
			opts.noCreate = true
		case 't':
			return handleTFlag(flags[j+1:], args, idx, opts, stderr)
		default:
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, flags[j])
			printTryHelp(stderr)
			return 1
		}
	}
	return -1
}

// handleTFlag processes the -t STAMP option. rest is any remaining
// characters after 't' in the current flag group.
func handleTFlag(rest string, args []string, idx *int, opts *touchOpts, stderr io.Writer) int {
	stamp := rest
	if stamp == "" {
		*idx++
		if *idx >= len(args) {
			fmt.Fprintf(stderr, "%s: option requires an argument -- 't'\n", progName)
			printTryHelp(stderr)
			return 1
		}
		stamp = args[*idx]
	}
	t, err := parseStamp(stamp)
	if err != nil {
		fmt.Fprintf(stderr, "%s: invalid date format '%s'\n", progName, stamp)
		return 1
	}
	opts.timestamp = &t
	return -1
}

// parseStamp parses a -t timestamp in [[CC]YY]MMDDhhmm[.ss] format.
// R2.4: explicit timestamp specification.
func parseStamp(s string) (time.Time, error) {
	base, sec, err := splitStampSeconds(s)
	if err != nil {
		return time.Time{}, err
	}
	year, mon, day, hour, min, err := parseStampBase(base)
	if err != nil {
		return time.Time{}, err
	}
	t := time.Date(year, time.Month(mon), day, hour, min, sec, 0, time.Local)
	if t.Day() != day || t.Month() != time.Month(mon) {
		return time.Time{}, fmt.Errorf("invalid date")
	}
	return t, nil
}

// splitStampSeconds separates the optional .ss suffix from a -t stamp.
func splitStampSeconds(s string) (string, int, error) {
	idx := strings.LastIndex(s, ".")
	if idx < 0 {
		return s, 0, nil
	}
	secStr := s[idx+1:]
	if len(secStr) != 2 {
		return "", 0, fmt.Errorf("invalid seconds")
	}
	sec, err := strconv.Atoi(secStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid seconds")
	}
	return s[:idx], sec, nil
}

// parseStampBase parses the base portion (without .ss) of a -t stamp.
func parseStampBase(base string) (year, mon, day, hour, min int, err error) {
	for _, c := range base {
		if c < '0' || c > '9' {
			return 0, 0, 0, 0, 0, fmt.Errorf("non-digit in timestamp")
		}
	}
	switch len(base) {
	case 8: // MMDDhhmm
		return time.Now().Year(), atoi2(base, 0), atoi2(base, 2),
			atoi2(base, 4), atoi2(base, 6), nil
	case 10: // YYMMDDhhmm
		year = expandTwoDigitYear(atoi2(base, 0))
		return year, atoi2(base, 2), atoi2(base, 4),
			atoi2(base, 6), atoi2(base, 8), nil
	case 12: // CCYYMMDDhhmm
		return atoi4(base, 0), atoi2(base, 4), atoi2(base, 6),
			atoi2(base, 8), atoi2(base, 10), nil
	default:
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid timestamp length")
	}
}

// atoi2 parses a 2-digit decimal from s at offset.
func atoi2(s string, off int) int {
	return int(s[off]-'0')*10 + int(s[off+1]-'0')
}

// atoi4 parses a 4-digit decimal from s at offset.
func atoi4(s string, off int) int {
	return int(s[off]-'0')*1000 + int(s[off+1]-'0')*100 +
		int(s[off+2]-'0')*10 + int(s[off+3]-'0')
}

// expandTwoDigitYear converts a 2-digit year to 4-digit.
// 69-99 → 1969-1999, 00-68 → 2000-2068. Matches POSIX convention.
func expandTwoDigitYear(yy int) int {
	if yy >= 69 {
		return 1900 + yy
	}
	return 2000 + yy
}

// finishParse validates that at least one file was provided.
func finishParse(opts touchOpts, stderr io.Writer) (touchOpts, int) {
	if len(opts.files) == 0 {
		fmt.Fprintf(stderr, "%s: missing file operand\n", progName)
		printTryHelp(stderr)
		return touchOpts{}, 1
	}
	return opts, -1
}

// stripPathError extracts the underlying message from a *os.PathError.
func stripPathError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... FILE...\n", progName)
	fmt.Fprintln(w, "Update the access and modification times of each FILE to the current time.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "A FILE argument that does not exist is created empty.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -a                 change only the access time")
	fmt.Fprintln(w, "  -c, --no-create    do not create any files")
	fmt.Fprintln(w, "  -m                 change only the modification time")
	fmt.Fprintln(w, "  -t STAMP           use [[CC]YY]MMDDhhmm[.ss] instead of current time")
	fmt.Fprintln(w, "      --help         display this help and exit")
	fmt.Fprintln(w, "      --version      output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}
