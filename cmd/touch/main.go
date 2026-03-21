// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd062-touch R1.1–R1.4: default file creation,
// timestamp update, multiple file arguments, and -c/--no-create.
// Implements prd062-touch R2.1–R2.4: -a (access only), -m (modification only),
// -t STAMP (explicit timestamp).
// Implements prd062-touch R3.1–R3.4: -r/--reference (reference file),
// -d/--date (date string), missing reference error, -h/--no-dereference.
// Implements prd062-touch R4.1–R4.4: exit 0 on success, exit 1 on error
// with continuation, differential tests for exit codes and timestamps.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "touch"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr, time.Now()))
}

// touchOpts holds parsed command-line options for the touch utility.
type touchOpts struct {
	noCreate      bool
	accessOnly    bool       // R2.1: -a flag
	modOnly       bool       // R2.2: -m flag
	noDereference bool       // R3.4: -h flag
	timestamp     *time.Time // R2.4: -t STAMP
	refFile       string     // R3.1: -r FILE
	dateStr       string     // R3.2: -d STRING
	files         []string
}

// changeBoth returns true when both access and modification times
// should be changed. R2.3: when neither -a nor -m is given, change both.
// Also change both when both -a and -m are given.
func (o touchOpts) changeBoth() bool {
	return (!o.accessOnly && !o.modOnly) || (o.accessOnly && o.modOnly)
}

// fileTimes holds the access and modification times of a file.
type fileTimes struct {
	atime time.Time
	mtime time.Time
}

// statTimes reads access and modification times from a file.
// R3.4: uses Lstat when noDereference is set.
func statTimes(path string, noDereference bool) (fileTimes, error) {
	var fi *sys.FileInfo
	var err error
	if noDereference {
		fi, err = sys.Lstat(path)
	} else {
		fi, err = sys.Stat(path)
	}
	if err != nil {
		return fileTimes{}, err
	}
	return fileTimes{atime: fi.AccessTime, mtime: fi.ModTime}, nil
}

// effectiveTimes resolves the timestamps to apply. R3.1: reference file.
// R3.2: date string. R2.4: -t stamp. Default: current time.
func effectiveTimes(opts touchOpts, now time.Time) (fileTimes, error) {
	if opts.refFile != "" {
		return statTimes(opts.refFile, opts.noDereference)
	}
	if opts.dateStr != "" {
		t, err := parseDateString(opts.dateStr)
		if err != nil {
			return fileTimes{}, err
		}
		return fileTimes{atime: t, mtime: t}, nil
	}
	t := now
	if opts.timestamp != nil {
		t = *opts.timestamp
	}
	return fileTimes{atime: t, mtime: t}, nil
}

// run parses arguments and processes each file.
func run(args []string, stderr io.Writer, now time.Time) int {
	opts, code := parseArgs(args, stderr)
	if code >= 0 {
		return code
	}
	times, err := effectiveTimes(opts, now)
	if err != nil {
		reportTimeError(opts, err, stderr)
		return 1
	}
	exitCode := 0
	for _, file := range opts.files {
		if err := touchFile(file, times, opts, stderr); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// touchFile creates or updates timestamps for a single file.
// R3.4: uses Lstat when noDereference is set.
func touchFile(path string, times fileTimes, opts touchOpts, stderr io.Writer) error {
	var err error
	if opts.noDereference {
		_, err = os.Lstat(path)
	} else {
		_, err = os.Stat(path)
	}
	if os.IsNotExist(err) {
		return handleMissing(path, times, opts, stderr)
	}
	if err != nil {
		reportError(path, stripPathError(err), stderr)
		return err
	}
	return applyTimestamps(path, times, opts, stderr)
}

// handleMissing creates the file or skips it based on noCreate and noDereference.
// R1.2: create empty file. R1.3: suppress with -c.
// R3.4: -h without -c on nonexistent file reports an error (utimensat cannot create).
func handleMissing(path string, times fileTimes, opts touchOpts, stderr io.Writer) error {
	if opts.noCreate {
		return nil
	}
	if opts.noDereference {
		reportSetTimesError(path, "No such file or directory", stderr)
		return fmt.Errorf("no such file or directory")
	}
	f, err := os.Create(path)
	if err != nil {
		reportError(path, stripPathError(err), stderr)
		return err
	}
	f.Close() // best-effort close, file created successfully
	return applyTimestamps(path, times, opts, stderr)
}

// applyTimestamps sets access and/or modification times based on opts.
// R2.1: -a changes only access time.
// R2.2: -m changes only modification time.
// R2.3: neither -a nor -m changes both.
func applyTimestamps(path string, times fileTimes, opts touchOpts, stderr io.Writer) error {
	atime, mtime := times.atime, times.mtime
	if !opts.changeBoth() {
		cur, err := statTimes(path, opts.noDereference)
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
	if err := setTimestamps(path, atime, mtime, opts.noDereference); err != nil {
		reportError(path, stripPathError(err), stderr)
		return err
	}
	return nil
}

// setTimestamps sets file timestamps, using utimensat for symlinks.
// R3.4: AT_SYMLINK_NOFOLLOW when noDereference is set.
func setTimestamps(path string, atime, mtime time.Time, noDereference bool) error {
	if noDereference {
		ts := []unix.Timespec{
			unix.NsecToTimespec(atime.UnixNano()),
			unix.NsecToTimespec(mtime.UnixNano()),
		}
		return unix.UtimesNanoAt(unix.AT_FDCWD, path, ts, unix.AT_SYMLINK_NOFOLLOW)
	}
	return os.Chtimes(path, atime, mtime)
}

// reportError prints a standardized error message to stderr.
func reportError(path, msg string, stderr io.Writer) {
	fmt.Fprintf(stderr, "%s: cannot touch '%s': %s\n", progName, path, msg)
}

// reportSetTimesError prints the "setting times of" error used by -h on
// nonexistent files, matching GNU touch error format.
func reportSetTimesError(path, msg string, stderr io.Writer) {
	fmt.Fprintf(stderr, "%s: setting times of '%s': %s\n", progName, path, msg)
}

// reportTimeError prints an error for time resolution failures.
// R3.3: reference file not found. R3.2: invalid date string.
func reportTimeError(opts touchOpts, err error, stderr io.Writer) {
	if opts.refFile != "" {
		fmt.Fprintf(stderr, "%s: failed to get attributes of '%s': %s\n",
			progName, opts.refFile, stripPathError(err))
		return
	}
	fmt.Fprintf(stderr, "%s: invalid date format '%s'\n", progName, opts.dateStr)
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
		case strings.HasPrefix(arg, "--"):
			code := parseLongFlag(arg, args, &i, &opts, stderr)
			if code >= 0 {
				return touchOpts{}, code
			}
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

// parseLongFlag handles a single -- prefixed flag.
func parseLongFlag(arg string, args []string, idx *int, opts *touchOpts, stderr io.Writer) int {
	switch {
	case arg == "--no-create":
		opts.noCreate = true
	case arg == "--no-dereference":
		opts.noDereference = true
	case arg == "--help":
		printHelp(os.Stdout)
		return 0
	case arg == "--version":
		printVersion(os.Stdout)
		return 0
	case arg == "--reference" || strings.HasPrefix(arg, "--reference="):
		val, code := consumeLongArg("--reference", arg, args, idx, stderr)
		if code >= 0 {
			return code
		}
		opts.refFile = val
	case arg == "--date" || strings.HasPrefix(arg, "--date="):
		val, code := consumeLongArg("--date", arg, args, idx, stderr)
		if code >= 0 {
			return code
		}
		opts.dateStr = val
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
	return -1
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
		case 'h':
			opts.noDereference = true
		case 't':
			return handleTFlag(flags[j+1:], args, idx, opts, stderr)
		case 'r':
			val, code := consumeFlagArg('r', flags[j+1:], args, idx, stderr)
			if code >= 0 {
				return code
			}
			opts.refFile = val
			return -1
		case 'd':
			val, code := consumeFlagArg('d', flags[j+1:], args, idx, stderr)
			if code >= 0 {
				return code
			}
			opts.dateStr = val
			return -1
		default:
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, flags[j])
			printTryHelp(stderr)
			return 1
		}
	}
	return -1
}

// consumeFlagArg extracts the argument value for a short flag.
// rest is any remaining characters after the flag letter.
func consumeFlagArg(flag byte, rest string, args []string, idx *int, stderr io.Writer) (string, int) {
	if rest != "" {
		return rest, -1
	}
	*idx++
	if *idx >= len(args) {
		fmt.Fprintf(stderr, "%s: option requires an argument -- '%c'\n", progName, flag)
		printTryHelp(stderr)
		return "", 1
	}
	return args[*idx], -1
}

// consumeLongArg extracts the argument value for a long flag.
// Supports both --flag=value and --flag value forms.
func consumeLongArg(name, arg string, args []string, idx *int, stderr io.Writer) (string, int) {
	if _, val, ok := strings.Cut(arg, "="); ok {
		return val, -1
	}
	*idx++
	if *idx >= len(args) {
		fmt.Fprintf(stderr, "%s: option '%s' requires an argument\n", progName, name)
		printTryHelp(stderr)
		return "", 1
	}
	return args[*idx], -1
}

// handleTFlag processes the -t STAMP option.
func handleTFlag(rest string, args []string, idx *int, opts *touchOpts, stderr io.Writer) int {
	stamp, code := consumeFlagArg('t', rest, args, idx, stderr)
	if code >= 0 {
		return code
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

// dateLayouts lists date string formats accepted by -d. R3.2.
var dateLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

// parseDateString parses a -d date string. R3.2.
// Supports ISO 8601, date-only, and @epoch formats.
func parseDateString(s string) (time.Time, error) {
	if strings.HasPrefix(s, "@") {
		return parseEpoch(s[1:])
	}
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date format")
}

// parseEpoch parses @SECONDS[.NANOSECONDS] epoch format.
func parseEpoch(s string) (time.Time, error) {
	parts := strings.SplitN(s, ".", 2)
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	var nsec int64
	if len(parts) == 2 {
		nsec, err = parseNanoseconds(parts[1])
		if err != nil {
			return time.Time{}, err
		}
	}
	return time.Unix(sec, nsec), nil
}

// parseNanoseconds pads a fractional seconds string to 9 digits and parses.
func parseNanoseconds(frac string) (int64, error) {
	for len(frac) < 9 {
		frac += "0"
	}
	return strconv.ParseInt(frac[:9], 10, 64)
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

// stripPathError extracts the underlying syscall message from an error chain
// that may contain *os.PathError (possibly wrapped by fmt.Errorf).
func stripPathError(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
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
	fmt.Fprintln(w, "  -a                     change only the access time")
	fmt.Fprintln(w, "  -c, --no-create        do not create any files")
	fmt.Fprintln(w, "  -d, --date=STRING      parse STRING and use it instead of current time")
	fmt.Fprintln(w, "  -h, --no-dereference   affect each symbolic link instead of any referenced file")
	fmt.Fprintln(w, "  -m                     change only the modification time")
	fmt.Fprintln(w, "  -r, --reference=FILE   use this file's times instead of current time")
	fmt.Fprintln(w, "  -t STAMP               use [[CC]YY]MMDDhhmm[.ss] instead of current time")
	fmt.Fprintln(w, "      --help             display this help and exit")
	fmt.Fprintln(w, "      --version          output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}
