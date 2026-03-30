// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/touch implements GNU touch: create files and update timestamps.
//
// Implements prd062-touch R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// touchOptions holds parsed flag state.
type touchOptions struct {
	noCreate      bool   // -c, --no-create: do not create files
	accessOnly    bool   // R2.1: -a changes only access time
	modOnly       bool   // R2.2: -m changes only modification time
	stamp         string // R2.4: -t [[CC]YY]MMDDhhmm[.ss]
	refFile       string // R3.1: -r FILE, --reference=FILE
	dateStr       string // R3.2: -d STRING, --date=STRING
	noDereference bool   // R3.4: -h, --no-dereference
}

// resolvedTime holds the resolved atime and mtime to apply.
type resolvedTime struct {
	atime time.Time
	mtime time.Time
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

	rt, err := resolveTimestamp(opts, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "touch: %v\n", err)
		return 1
	}

	exitCode := 0
	for _, f := range files {
		if err := touchFile(f, rt, opts); err != nil {
			fmt.Fprintf(stderr, "touch: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// resolveTimestamp determines the atime and mtime to apply.
// R3.1: -r uses reference file timestamps.
// R3.2: -d parses date string.
// R2.4: -t uses stamp format.
func resolveTimestamp(opts touchOptions, now time.Time) (resolvedTime, error) {
	if opts.refFile != "" {
		return resolveFromRef(opts.refFile)
	}
	if opts.dateStr != "" {
		return resolveFromDate(opts.dateStr)
	}
	if opts.stamp != "" {
		t, err := parseStamp(opts.stamp, now)
		if err != nil {
			return resolvedTime{}, err
		}
		return resolvedTime{atime: t, mtime: t}, nil
	}
	return resolvedTime{atime: now, mtime: now}, nil
}

// resolveFromRef reads timestamps from the reference file.
// R3.3: returns error if reference file does not exist.
func resolveFromRef(path string) (resolvedTime, error) {
	fi, err := sys.Stat(path)
	if err != nil {
		return resolvedTime{}, fmt.Errorf(
			"failed to get attributes of %q: %v", path, err)
	}
	return resolvedTime{atime: fi.AccessTime, mtime: fi.ModTime}, nil
}

// resolveFromDate parses a date string into resolved timestamps.
func resolveFromDate(s string) (resolvedTime, error) {
	t, err := parseDateString(s)
	if err != nil {
		return resolvedTime{}, err
	}
	return resolvedTime{atime: t, mtime: t}, nil
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
		if strings.HasPrefix(arg, "--") {
			i += parseLongFlag(&opts, arg, args, i)
			continue
		}
		if len(arg) >= 2 && arg[0] == '-' {
			target := parseShortFlags(&opts, arg[1:])
			if target != nil && i+1 < len(args) {
				i++
				*target = args[i]
			}
			continue
		}
		files = append(files, arg)
	}
	return opts, files
}

// parseLongFlag handles --flag and --flag=VALUE forms.
// Returns the number of additional args consumed.
func parseLongFlag(opts *touchOptions, arg string, args []string, idx int) int {
	switch {
	case arg == "--no-create":
		opts.noCreate = true
	case arg == "--no-dereference":
		opts.noDereference = true
	case strings.HasPrefix(arg, "--reference="):
		opts.refFile = arg[len("--reference="):]
	case arg == "--reference" && idx+1 < len(args):
		opts.refFile = args[idx+1]
		return 1
	case strings.HasPrefix(arg, "--date="):
		opts.dateStr = arg[len("--date="):]
	case arg == "--date" && idx+1 < len(args):
		opts.dateStr = args[idx+1]
		return 1
	}
	return 0
}

// parseShortFlags processes short flag characters.
// Returns a pointer to the field needing the next argument, or nil.
func parseShortFlags(opts *touchOptions, chars string) *string {
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
		case 'h':
			// R3.4: affect symlink itself
			opts.noDereference = true
		case 't', 'r', 'd':
			return consumeArgFlag(opts, ch, chars[i+1:])
		}
	}
	return nil
}

// consumeArgFlag handles -t, -r, -d which take a value argument.
// If remaining chars exist after the flag, they are the value.
func consumeArgFlag(opts *touchOptions, ch rune, rest string) *string {
	target := shortFlagTarget(opts, ch)
	if rest != "" {
		*target = rest
		return nil
	}
	return target
}

// shortFlagTarget returns a pointer to the option field for the flag.
func shortFlagTarget(opts *touchOptions, ch rune) *string {
	switch ch {
	case 't':
		return &opts.stamp
	case 'r':
		return &opts.refFile
	default:
		return &opts.dateStr
	}
}

// parseDateString parses a -d date string.
// R3.2: supports @epoch and ISO 8601 formats.
func parseDateString(s string) (time.Time, error) {
	if strings.HasPrefix(s, "@") {
		return parseEpoch(s[1:])
	}
	return tryDateLayouts(s)
}

// parseEpoch parses @SECONDS[.NANOSECONDS] epoch format.
func parseEpoch(s string) (time.Time, error) {
	dot := strings.Index(s, ".")
	if dot < 0 {
		sec, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date %q", "@"+s)
		}
		return time.Unix(sec, 0), nil
	}
	return parseEpochFractional(s, dot)
}

// parseEpochFractional parses epoch with fractional seconds.
func parseEpochFractional(s string, dot int) (time.Time, error) {
	sec, err := strconv.ParseInt(s[:dot], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q", "@"+s)
	}
	nsecStr := (s[dot+1:] + "000000000")[:9]
	nsec, err := strconv.ParseInt(nsecStr, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q", "@"+s)
	}
	return time.Unix(sec, nsec), nil
}

// dateLayouts lists ISO 8601 formats for tryDateLayouts.
var dateLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// tryDateLayouts attempts to parse s with each layout in local time.
func tryDateLayouts(s string) (time.Time, error) {
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date format %q", s)
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
// R1.1: updates atime and mtime.
// R1.2: creates the file if it does not exist.
// R1.3: skips creation when noCreate is set.
func touchFile(path string, rt resolvedTime, opts touchOptions) error {
	_, err := statForCheck(path, opts.noDereference)
	if os.IsNotExist(err) {
		if opts.noCreate {
			return nil
		}
		return createAndTouch(path, rt, opts)
	}
	if err != nil {
		return err
	}
	return applyTimestamps(path, rt, opts)
}

// statForCheck checks file existence, respecting -h for symlinks.
func statForCheck(path string, noDeref bool) (os.FileInfo, error) {
	if noDeref {
		return os.Lstat(path)
	}
	return os.Stat(path)
}

// createAndTouch creates an empty file and sets its timestamps.
func createAndTouch(path string, rt resolvedTime, opts touchOptions) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	f.Close() // best-effort; error on applyTimestamps is more important
	return applyTimestamps(path, rt, opts)
}

// applyTimestamps sets atime and/or mtime based on -a/-m flags.
// R2.1: -a alone changes only access time, preserving mtime.
// R2.2: -m alone changes only modification time, preserving atime.
// R2.3: neither -a nor -m (or both) changes both times.
// R3.4: uses lchtimes when -h is set.
func applyTimestamps(path string, rt resolvedTime, opts touchOptions) error {
	atime, mtime := rt.atime, rt.mtime
	if opts.accessOnly != opts.modOnly {
		fi, err := sysStatFile(path, opts.noDereference)
		if err != nil {
			return err
		}
		if opts.accessOnly {
			mtime = fi.ModTime
		} else {
			atime = fi.AccessTime
		}
	}
	if opts.noDereference {
		return lchtimes(path, atime, mtime)
	}
	return os.Chtimes(path, atime, mtime)
}

// sysStatFile reads extended file info, respecting -h for symlinks.
func sysStatFile(path string, noDeref bool) (*sys.FileInfo, error) {
	if noDeref {
		return sys.Lstat(path)
	}
	return sys.Stat(path)
}

// lchtimes changes timestamps of a symlink without following it.
// R3.4: uses AT_SYMLINK_NOFOLLOW to affect the link itself.
func lchtimes(path string, atime, mtime time.Time) error {
	ts := []unix.Timespec{
		unix.NsecToTimespec(atime.UnixNano()),
		unix.NsecToTimespec(mtime.UnixNano()),
	}
	return unix.UtimesNanoAt(unix.AT_FDCWD, path, ts, unix.AT_SYMLINK_NOFOLLOW)
}
