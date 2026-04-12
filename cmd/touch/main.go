// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/touch: create files and update timestamps.
// Implements srd062-touch R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

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
	noCreate      bool
	accessOnly    bool   // R2.1: -a flag
	modOnly       bool   // R2.2: -m flag
	noDereference bool   // R3.4: -h flag
	stamp         string // R2.4: -t STAMP value
	refFile       string // R3.1: -r FILE value
	dateStr       string // R3.2: -d STRING value
	files         []string
}

// resolvedTime holds the timestamps to apply to target files.
type resolvedTime struct {
	atime time.Time
	mtime time.Time
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the touch logic and returns the exit code.
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

// resolveTime determines the timestamps to apply.
// R2.3: default is current time. R2.4: -t overrides.
// R3.1: -r overrides with reference file timestamps.
// R3.2: -d overrides with parsed date string.
func resolveTime(opts *options) (resolvedTime, error) {
	if opts.refFile != "" {
		return resolveRefTime(opts.refFile)
	}
	if opts.dateStr != "" {
		return resolveDateStr(opts.dateStr)
	}
	if opts.stamp != "" {
		return resolveStamp(opts.stamp)
	}
	now := time.Now()
	return resolvedTime{atime: now, mtime: now}, nil
}

// resolveRefTime reads timestamps from a reference file.
// R3.1: use reference file timestamps.
// R3.3: error if reference file does not exist.
func resolveRefTime(refFile string) (resolvedTime, error) {
	fi, err := sys.Stat(refFile)
	if err != nil {
		return resolvedTime{}, fmt.Errorf(
			"failed to get attributes of '%s': %s", refFile, sysError(err))
	}
	return resolvedTime{atime: fi.AccessTime, mtime: fi.ModTime}, nil
}

// resolveDateStr parses a date string and returns it as both timestamps.
// R3.2: -d STRING parsing.
func resolveDateStr(dateStr string) (resolvedTime, error) {
	t, err := parseDate(dateStr)
	if err != nil {
		return resolvedTime{}, err
	}
	return resolvedTime{atime: t, mtime: t}, nil
}

// resolveStamp parses a -t STAMP value and returns it as both timestamps.
func resolveStamp(stamp string) (resolvedTime, error) {
	t, err := parseStamp(stamp)
	if err != nil {
		return resolvedTime{}, err
	}
	return resolvedTime{atime: t, mtime: t}, nil
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
		if arg == "--no-dereference" {
			opts.noDereference = true
			i++
			continue
		}
		if handled, advance, err := parseLongWithValue(arg, args, i, opts); handled {
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
// R3.1: --reference=FILE. R3.2: --date=STRING.
func parseLongWithValue(arg string, args []string, idx int, opts *options) (bool, int, error) {
	if strings.HasPrefix(arg, "--date=") {
		opts.dateStr = arg[len("--date="):]
		return true, 1, nil
	}
	if strings.HasPrefix(arg, "--reference=") {
		opts.refFile = arg[len("--reference="):]
		return true, 1, nil
	}
	if arg == "--date" {
		if idx+1 >= len(args) {
			return true, 1, fmt.Errorf("option '%s' requires an argument", arg)
		}
		opts.dateStr = args[idx+1]
		return true, 2, nil
	}
	if arg == "--reference" {
		if idx+1 >= len(args) {
			return true, 1, fmt.Errorf("option '%s' requires an argument", arg)
		}
		opts.refFile = args[idx+1]
		return true, 2, nil
	}
	return false, 0, nil
}

// parseShortFlags processes a cluster of short flags (e.g., "-acm").
// R1.3: -c. R2.1: -a. R2.2: -m. R2.4: -t.
// R3.1: -r. R3.2: -d. R3.4: -h.
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
			opts.noDereference = true
		case 't':
			return consumeValueFlag(flags, args, idx, j, 't', &opts.stamp)
		case 'r':
			return consumeValueFlag(flags, args, idx, j, 'r', &opts.refFile)
		case 'd':
			return consumeValueFlag(flags, args, idx, j, 'd', &opts.dateStr)
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
// R1.3: -c suppresses creation. R3.4: -h uses lstat and suppresses creation.
func touchFile(path string, opts *options, ts resolvedTime) error {
	statFn := os.Stat
	if opts.noDereference {
		statFn = os.Lstat
	}
	_, err := statFn(path)
	if os.IsNotExist(err) {
		if opts.noCreate || opts.noDereference {
			return nil // R1.3, R3.4: suppress creation silently
		}
		if err := createEmpty(path); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("cannot touch '%s': %s", path, sysError(err))
	}
	return applyTimestamps(path, opts, ts)
}

// createEmpty creates an empty file with default permissions.
// R1.2: create file as empty with default permissions.
func createEmpty(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot touch '%s': %s", path, sysError(err))
	}
	f.Close() // best-effort close; file is empty
	return nil
}

// applyTimestamps sets atime and/or mtime based on -a/-m flags.
// R2.1: -a changes only access time.
// R2.2: -m changes only modification time.
// R2.3: neither -a nor -m changes both.
// R3.4: -h uses lstat and UtimesNanoAt with AT_SYMLINK_NOFOLLOW.
func applyTimestamps(path string, opts *options, ts resolvedTime) error {
	atime, mtime, err := computeTimestamps(path, opts, ts)
	if err != nil {
		return err
	}
	if opts.noDereference {
		return setTimesNoFollow(path, atime, mtime)
	}
	if err := os.Chtimes(path, atime, mtime); err != nil {
		return fmt.Errorf("cannot touch '%s': %s", path, sysError(err))
	}
	return nil
}

// computeTimestamps determines final atime/mtime applying -a/-m selection.
func computeTimestamps(path string, opts *options, ts resolvedTime) (time.Time, time.Time, error) {
	var fi *sys.FileInfo
	var err error
	if opts.noDereference {
		fi, err = sys.Lstat(path)
	} else {
		fi, err = sys.Stat(path)
	}
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("cannot touch '%s': %s", path, sysError(err))
	}
	atime := fi.AccessTime
	mtime := fi.ModTime
	changeAccess := !opts.modOnly || opts.accessOnly
	changeMod := !opts.accessOnly || opts.modOnly
	if changeAccess {
		atime = ts.atime
	}
	if changeMod {
		mtime = ts.mtime
	}
	return atime, mtime, nil
}

// setTimesNoFollow sets timestamps on a symlink without following it.
// R3.4: uses UtimesNanoAt with AT_SYMLINK_NOFOLLOW.
func setTimesNoFollow(path string, atime, mtime time.Time) error {
	ts := [2]unix.Timespec{
		unix.NsecToTimespec(atime.UnixNano()),
		unix.NsecToTimespec(mtime.UnixNano()),
	}
	err := unix.UtimesNanoAt(unix.AT_FDCWD, path, ts[:], unix.AT_SYMLINK_NOFOLLOW)
	if err != nil {
		return fmt.Errorf("cannot touch '%s': %s", path, capitalizeFirst(err.Error()))
	}
	return nil
}

// sysError extracts the underlying OS error message from a *os.PathError
// or *os.SyscallError, producing output like "No such file or directory"
// instead of "stat /path: no such file or directory".
func sysError(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return capitalizeFirst(pe.Err.Error())
	}
	return capitalizeFirst(err.Error())
}

// capitalizeFirst uppercases the first letter of a string to match GNU error messages.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parseDate parses a date string for the -d flag.
// R3.2: supports ISO 8601 formats and @epoch notation.
func parseDate(s string) (time.Time, error) {
	if strings.HasPrefix(s, "@") {
		return parseEpoch(s[1:])
	}
	return parseDateLayouts(s)
}

// dateLayoutTZ contains layouts with explicit timezone info.
var dateLayoutTZ = []string{
	"2006-01-02 15:04:05.999999999 -0700",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02T15:04:05.999999999Z",
	"2006-01-02T15:04:05Z",
}

// dateLayoutLocal contains layouts without timezone (interpreted as local).
var dateLayoutLocal = []string{
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// parseDateLayouts tries to parse s against known date layouts.
func parseDateLayouts(s string) (time.Time, error) {
	for _, layout := range dateLayoutTZ {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	for _, layout := range dateLayoutLocal {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date format '%s'", s)
}

// parseEpoch parses an epoch seconds string (from @SECONDS[.FRAC]).
func parseEpoch(s string) (time.Time, error) {
	idx := strings.IndexByte(s, '.')
	if idx < 0 {
		sec, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date format '@%s'", s)
		}
		return time.Unix(sec, 0), nil
	}
	return parseEpochFrac(s, idx)
}

// parseEpochFrac parses epoch seconds with a fractional part.
func parseEpochFrac(s string, dotIdx int) (time.Time, error) {
	sec, err := strconv.ParseInt(s[:dotIdx], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format '@%s'", s)
	}
	frac := s[dotIdx+1:]
	for len(frac) < 9 {
		frac += "0"
	}
	nsec, err := strconv.ParseInt(frac[:9], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format '@%s'", s)
	}
	return time.Unix(sec, nsec), nil
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
