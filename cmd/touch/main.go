// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd062-touch: Create Files and Update Timestamps.
// Covers R1.1-R1.4 (entry point, file creation, -c no-create, multiple files),
// R2.1-R2.2 (-a access only, -m modification only),
// R2.3-R2.4 (default both times, -t POSIX timestamp),
// R3.1-R3.2 (-r reference file, -d date string),
// R3.3 (error on missing reference), R3.4 (-h no-dereference).
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// config holds parsed command-line options.
type config struct {
	accessOnly  bool
	modOnly     bool
	noCreate    bool
	noDeref     bool
	refFile     string
	stampStr    string
	dateStr     string
	showHelp    bool
	showVersion bool
	files       []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and processes files. Returns exit code.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		printTryHelp()
		return 1
	}
	if cfg.showHelp {
		return printHelp()
	}
	if cfg.showVersion {
		return printVersion()
	}
	if len(cfg.files) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\n", progName)
		printTryHelp()
		return 1
	}
	return touchFiles(cfg)
}

// touchFiles processes each file in the config. R1.4, R4.1, R4.2.
func touchFiles(cfg config) int {
	ts, err := resolveTimestamp(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	exitCode := 0
	for _, f := range cfg.files {
		if err := touchOne(cfg, ts, f); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// timestamps holds the resolved access and modification times.
type timestamps struct {
	atime time.Time
	mtime time.Time
}

// resolveTimestamp determines the timestamp to apply based on config.
// R2.4: -t stamp. R3.2: -d date string. R3.1: -r reference file.
func resolveTimestamp(cfg config) (timestamps, error) {
	now := time.Now()
	if cfg.stampStr != "" {
		t, err := parseStamp(cfg.stampStr)
		if err != nil {
			return timestamps{}, err
		}
		return timestamps{atime: t, mtime: t}, nil
	}
	if cfg.dateStr != "" {
		t, err := parseDateStr(cfg.dateStr)
		if err != nil {
			return timestamps{}, err
		}
		return timestamps{atime: t, mtime: t}, nil
	}
	if cfg.refFile != "" {
		return readRefTimestamps(cfg.refFile, cfg.noDeref)
	}
	return timestamps{atime: now, mtime: now}, nil
}

// touchOne creates or updates timestamps for a single file.
// R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.4.
func touchOne(cfg config, ts timestamps, path string) error {
	exists, curAtime, curMtime, err := statFile(path, cfg.noDeref)
	if err != nil {
		return fmt.Errorf("cannot touch '%s': %v", path, err)
	}
	if !exists {
		if cfg.noCreate {
			return nil // R1.3: -c suppresses creation, no error
		}
		if err := createFile(path); err != nil {
			return err
		}
		curAtime = time.Now()
		curMtime = curAtime
	}
	atime, mtime := selectTimes(cfg, ts, curAtime, curMtime)
	return applyTimestamps(path, atime, mtime, cfg.noDeref)
}

// statFile returns existence and current timestamps for a path.
func statFile(path string, noDeref bool) (bool, time.Time, time.Time, error) {
	var fi *sys.FileInfo
	var err error
	if noDeref {
		fi, err = sys.Lstat(path)
	} else {
		fi, err = sys.Stat(path)
	}
	if err != nil {
		if os.IsNotExist(err) {
			return false, time.Time{}, time.Time{}, nil
		}
		return false, time.Time{}, time.Time{}, unwrapPathErr(err)
	}
	return true, fi.AccessTime, fi.ModTime, nil
}

// unwrapPathErr extracts the underlying syscall error from *os.PathError
// to match GNU coreutils error message format (no "stat path:" prefix).
func unwrapPathErr(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// createFile creates an empty file with default permissions. R1.2.
func createFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return fmt.Errorf("cannot touch '%s': %v", path, unwrapPathErr(err))
	}
	return f.Close()
}

// selectTimes picks access and modification times based on -a/-m flags.
// R2.1: -a only changes access time. R2.2: -m only changes modification time.
// R2.3: default changes both.
func selectTimes(
	cfg config, ts timestamps, curAtime, curMtime time.Time,
) (time.Time, time.Time) {
	atime := ts.atime
	mtime := ts.mtime
	if cfg.accessOnly && !cfg.modOnly {
		mtime = curMtime // R2.1: keep current mtime
	}
	if cfg.modOnly && !cfg.accessOnly {
		atime = curAtime // R2.2: keep current atime
	}
	return atime, mtime
}

// applyTimestamps sets the access and modification times on a path.
// R3.4: -h uses lutimes to affect symlinks themselves.
func applyTimestamps(
	path string, atime, mtime time.Time, noDeref bool,
) error {
	atimeSpec := unix.NsecToTimespec(atime.UnixNano())
	mtimeSpec := unix.NsecToTimespec(mtime.UnixNano())
	utimes := [2]unix.Timespec{atimeSpec, mtimeSpec}
	flags := 0
	if noDeref {
		flags = unix.AT_SYMLINK_NOFOLLOW
	}
	err := unix.UtimesNanoAt(unix.AT_FDCWD, path, utimes[:], flags)
	if err != nil {
		return fmt.Errorf("setting times of '%s': %v", path, err)
	}
	return nil
}

// readRefTimestamps reads timestamps from a reference file. R3.1, R3.3.
func readRefTimestamps(path string, noDeref bool) (timestamps, error) {
	var fi *sys.FileInfo
	var err error
	if noDeref {
		fi, err = sys.Lstat(path)
	} else {
		fi, err = sys.Stat(path)
	}
	if err != nil {
		return timestamps{}, fmt.Errorf(
			"failed to get attributes of '%s': %v", path, unwrapPathErr(err))
	}
	return timestamps{atime: fi.AccessTime, mtime: fi.ModTime}, nil
}

// parseStamp parses a POSIX -t timestamp: [[CC]YY]MMDDhhmm[.ss].
// R2.4: supports 8, 10, or 12 digit date portions with optional .ss.
func parseStamp(s string) (time.Time, error) {
	datePart, secPart, err := splitStampParts(s)
	if err != nil {
		return time.Time{}, err
	}
	year, mon, day, hour, min, err := parseStampDate(datePart)
	if err != nil {
		return time.Time{}, err
	}
	sec := 0
	if secPart != "" {
		sec, err = strconv.Atoi(secPart)
		if err != nil || sec < 0 || sec > 61 {
			return time.Time{}, fmt.Errorf("invalid date format '%s'", s)
		}
	}
	t := time.Date(year, time.Month(mon), day, hour, min, sec, 0, time.Local)
	return t, nil
}

// splitStampParts separates the date and seconds portions of a -t stamp.
func splitStampParts(s string) (string, string, error) {
	dotIdx := strings.LastIndex(s, ".")
	if dotIdx < 0 {
		return s, "", nil
	}
	secStr := s[dotIdx+1:]
	if len(secStr) != 2 {
		return "", "", fmt.Errorf("invalid date format '%s'", s)
	}
	return s[:dotIdx], secStr, nil
}

// parseStampDate parses the date portion of a -t stamp into components.
func parseStampDate(s string) (int, int, int, int, int, error) {
	n := len(s)
	if n != 8 && n != 10 && n != 12 {
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format '%s'", s)
	}
	// Last 8 characters are always MMDDhhmm
	tail := s[n-8:]
	prefix := s[:n-8]
	mon, err := strconv.Atoi(tail[0:2])
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format '%s'", s)
	}
	day, err := strconv.Atoi(tail[2:4])
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format '%s'", s)
	}
	hour, err := strconv.Atoi(tail[4:6])
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format '%s'", s)
	}
	min, err := strconv.Atoi(tail[6:8])
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format '%s'", s)
	}
	year, err := resolveStampYear(prefix, s)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	return year, mon, day, hour, min, nil
}

// resolveStampYear determines the year from the prefix of a -t stamp.
func resolveStampYear(prefix, orig string) (int, error) {
	switch len(prefix) {
	case 0:
		return time.Now().Year(), nil
	case 2:
		yy, err := strconv.Atoi(prefix)
		if err != nil {
			return 0, fmt.Errorf("invalid date format '%s'", orig)
		}
		if yy >= 69 {
			return 1900 + yy, nil
		}
		return 2000 + yy, nil
	case 4:
		yyyy, err := strconv.Atoi(prefix)
		if err != nil {
			return 0, fmt.Errorf("invalid date format '%s'", orig)
		}
		return yyyy, nil
	default:
		return 0, fmt.Errorf("invalid date format '%s'", orig)
	}
}

// isoLayouts lists Go time layouts for ISO 8601 date string parsing.
var isoLayouts = []string{
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02 15:04:05 -07:00",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// parseDateStr parses a -d/--date date string. R3.2.
// Supports @epoch and ISO 8601 formats.
func parseDateStr(s string) (time.Time, error) {
	if strings.HasPrefix(s, "@") {
		return parseEpoch(s[1:])
	}
	return parseISO(s)
}

// parseEpoch parses a Unix epoch timestamp (@seconds).
func parseEpoch(s string) (time.Time, error) {
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date '@%s'", s)
	}
	return time.Unix(sec, 0), nil
}

// parseISO attempts to parse s as an ISO 8601 date string.
func parseISO(s string) (time.Time, error) {
	for _, layout := range isoLayouts {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date '%s'", s)
}

// parseArgs processes all command-line arguments into a config.
func parseArgs(args []string) (config, error) {
	var cfg config
	for i := 0; i < len(args); {
		if args[i] == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			return cfg, nil
		}
		adv, err := parseArg(&cfg, args, i)
		if err != nil {
			return cfg, err
		}
		i += adv
		if cfg.showHelp || cfg.showVersion {
			return cfg, nil
		}
	}
	return cfg, nil
}

// parseArg processes one argument, returning how many args were consumed.
func parseArg(cfg *config, args []string, i int) (int, error) {
	arg := args[i]
	switch {
	case arg == "--help":
		cfg.showHelp = true
		return 1, nil
	case arg == "--version":
		cfg.showVersion = true
		return 1, nil
	case arg == "--no-create":
		cfg.noCreate = true
		return 1, nil
	case arg == "--no-dereference":
		cfg.noDeref = true
		return 1, nil
	case strings.HasPrefix(arg, "--reference="):
		cfg.refFile = arg[len("--reference="):]
		return 1, nil
	case arg == "--reference":
		return consumeNextArg(&cfg.refFile, args, i, arg)
	case strings.HasPrefix(arg, "--date="):
		cfg.dateStr = arg[len("--date="):]
		return 1, nil
	case arg == "--date":
		return consumeNextArg(&cfg.dateStr, args, i, arg)
	case strings.HasPrefix(arg, "--"):
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlags(cfg, args, i)
	default:
		cfg.files = append(cfg.files, arg)
		return 1, nil
	}
}

// consumeNextArg sets dst to the argument following the current one.
func consumeNextArg(
	dst *string, args []string, i int, opt string,
) (int, error) {
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '%s' requires an argument", opt)
	}
	*dst = args[i+1]
	return 2, nil
}

// parseShortFlags processes combined short flags (e.g., -am, -t VALUE).
func parseShortFlags(cfg *config, args []string, i int) (int, error) {
	flags := args[i][1:]
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'a':
			cfg.accessOnly = true
		case 'm':
			cfg.modOnly = true
		case 'c':
			cfg.noCreate = true
		case 'h':
			cfg.noDeref = true
		case 'r':
			return consumeShortOptArg(
				&cfg.refFile, flags[j+1:], flags[j], args, i)
		case 't':
			return consumeShortOptArg(
				&cfg.stampStr, flags[j+1:], flags[j], args, i)
		case 'd':
			return consumeShortOptArg(
				&cfg.dateStr, flags[j+1:], flags[j], args, i)
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return 1, nil
}

// consumeShortOptArg sets dst from the flag tail or the next argument.
func consumeShortOptArg(
	dst *string, rest string, ch byte, args []string, i int,
) (int, error) {
	if rest != "" {
		*dst = rest
		return 1, nil
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- '%c'", ch)
	}
	*dst = args[i+1]
	return 2, nil
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

const helpText = `Usage: touch [OPTION]... FILE...
Update the access and modification times of each FILE to the current time.

A FILE argument that does not exist is created empty, unless -c is supplied.

Mandatory arguments to long options are mandatory for short options too.
  -a                     change only the access time
  -c, --no-create        do not create any files
  -d, --date=STRING      parse STRING and use it instead of current time
  -h, --no-dereference   affect each symbolic link instead of any referenced
                         file (useful only on systems that can change the
                         timestamps of a symlink)
  -m                     change only the modification time
  -r, --reference=FILE   use this file's times instead of current time
  -t STAMP               use [[CC]YY]MMDDhhmm[.ss] instead of current time
      --help             display this help and exit
      --version          output version information and exit
`

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := os.Stdout.WriteString(helpText)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(
		os.Stdout, "%s (go-unix-utils) %s\n", progName, version)
	if err != nil {
		return 1
	}
	return 0
}
