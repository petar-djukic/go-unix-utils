// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd062-touch R1.1-R1.4, R2.1-R2.4, R3.1-R3.4.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"golang.org/x/sys/unix"
)

type options struct {
	noCreate      bool
	accessOnly    bool
	modOnly       bool
	noDereference bool
	stampStr      string
	refFile       string
	dateStr       string
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "touch: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'touch --help' for more information.\n")
		os.Exit(1)
	}
	os.Exit(run(opts, files))
}

func run(opts options, files []string) int {
	t, err := resolveTime(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "touch: %s\n", err)
		return 1
	}
	exitCode := 0
	for _, path := range files {
		if err := touchFile(path, opts, t); err != nil {
			fmt.Fprintf(os.Stderr, "touch: %s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func resolveTime(opts options) (time.Time, error) {
	if opts.refFile != "" {
		return refFileTime(opts.refFile)
	}
	if opts.dateStr != "" {
		return parseDate(opts.dateStr)
	}
	if opts.stampStr != "" {
		return parseTimestamp(opts.stampStr)
	}
	return time.Now(), nil
}

func refFileTime(path string) (time.Time, error) {
	info, err := sys.Stat(path)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"failed to get attributes of '%s': %s", path, sysMsg(err))
	}
	return info.ModTime, nil
}

func touchFile(path string, opts options, t time.Time) error {
	statFn := os.Stat
	if opts.noDereference {
		statFn = os.Lstat
	}
	_, statErr := statFn(path)
	if os.IsNotExist(statErr) {
		if opts.noCreate {
			return nil
		}
		if !opts.noDereference {
			f, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("cannot touch '%s': %s", path, sysMsg(err))
			}
			f.Close()
		}
	}
	atime, mtime, err := resolveTimestamps(path, opts, t)
	if err != nil {
		return err
	}
	return setTimestamps(path, opts, atime, mtime)
}

func setTimestamps(path string, opts options, atime, mtime time.Time) error {
	if opts.noDereference {
		ts := []unix.Timespec{
			unix.NsecToTimespec(atime.UnixNano()),
			unix.NsecToTimespec(mtime.UnixNano()),
		}
		err := unix.UtimesNanoAt(
			unix.AT_FDCWD, path, ts, unix.AT_SYMLINK_NOFOLLOW)
		if err != nil {
			return fmt.Errorf("setting times of '%s': %s", path, err)
		}
		return nil
	}
	if err := os.Chtimes(path, atime, mtime); err != nil {
		return fmt.Errorf("setting times of '%s': %s", path, sysMsg(err))
	}
	return nil
}

func resolveTimestamps(path string, opts options, t time.Time) (time.Time, time.Time, error) {
	if !opts.accessOnly && !opts.modOnly {
		return t, t, nil
	}
	var info *sys.FileInfo
	var err error
	if opts.noDereference {
		info, err = sys.Lstat(path)
	} else {
		info, err = sys.Stat(path)
	}
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf(
			"cannot touch '%s': %s", path, sysMsg(err))
	}
	atime, mtime := info.AccessTime, info.ModTime
	if opts.accessOnly {
		atime = t
	}
	if opts.modOnly {
		mtime = t
	}
	return atime, mtime, nil
}

func sysMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var files []string
	endFlags := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if endFlags || arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			i++
			continue
		}
		if arg == "--" {
			endFlags = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--") {
			consumed, err := parseLongFlag(arg, args, i, &opts)
			if err != nil {
				return options{}, nil, err
			}
			i += 1 + consumed
			continue
		}
		consumed, err := parseShortFlags(arg[1:], args, i, &opts)
		if err != nil {
			return options{}, nil, err
		}
		i += 1 + consumed
	}
	if len(files) == 0 {
		return options{}, nil, fmt.Errorf("missing file operand")
	}
	return opts, files, nil
}

func parseLongFlag(flag string, args []string, idx int, opts *options) (int, error) {
	switch {
	case flag == "--no-create":
		opts.noCreate = true
		return 0, nil
	case flag == "--no-dereference":
		opts.noDereference = true
		return 0, nil
	case flag == "--reference" || strings.HasPrefix(flag, "--reference="):
		return handleLongValueFlag(
			flag, "--reference", args, idx, &opts.refFile)
	case flag == "--date" || strings.HasPrefix(flag, "--date="):
		return handleLongValueFlag(
			flag, "--date", args, idx, &opts.dateStr)
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func handleLongValueFlag(flag, name string, args []string, idx int, dest *string) (int, error) {
	if strings.Contains(flag, "=") {
		*dest = flag[len(name)+1:]
		return 0, nil
	}
	if idx+1 >= len(args) {
		return 0, fmt.Errorf("option '%s' requires an argument", name)
	}
	*dest = args[idx+1]
	return 1, nil
}

func parseShortFlags(flags string, args []string, argIdx int, opts *options) (int, error) {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'a':
			opts.accessOnly = true
		case 'c':
			opts.noCreate = true
		case 'h':
			opts.noDereference = true
		case 'm':
			opts.modOnly = true
		case 't':
			return handleValueFlag(flags[j+1:], args, argIdx, &opts.stampStr, 't')
		case 'r':
			return handleValueFlag(flags[j+1:], args, argIdx, &opts.refFile, 'r')
		case 'd':
			return handleValueFlag(flags[j+1:], args, argIdx, &opts.dateStr, 'd')
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return 0, nil
}

func handleValueFlag(remainder string, args []string, argIdx int, dest *string, ch byte) (int, error) {
	if remainder != "" {
		*dest = remainder
		return 0, nil
	}
	if argIdx+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- '%c'", ch)
	}
	*dest = args[argIdx+1]
	return 1, nil
}

func parseDate(s string) (time.Time, error) {
	if strings.HasPrefix(s, "@") {
		return parseEpoch(s[1:])
	}
	formats := []string{
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		"2006-01-02",
		"02 Jan 2006 15:04:05",
		"02 Jan 2006 15:04",
		"02 Jan 2006",
		"Jan 02 2006 15:04:05",
		"Jan 02 2006 15:04",
		"Jan 02, 2006 15:04:05",
		"Jan 02, 2006 15:04",
		"Jan 02, 2006",
	}
	for _, layout := range formats {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date format '%s'", s)
}

func parseEpoch(s string) (time.Time, error) {
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format '@%s'", s)
	}
	return time.Unix(sec, 0), nil
}

func parseTimestamp(stamp string) (time.Time, error) {
	base, ss, err := splitSeconds(stamp)
	if err != nil {
		return time.Time{}, err
	}
	year, month, day, hour, min, err := parseDateFields(base, stamp)
	if err != nil {
		return time.Time{}, err
	}
	t := time.Date(year, time.Month(month), day, hour, min, ss, 0, time.Local)
	if t.Year() != year || int(t.Month()) != month || t.Day() != day ||
		t.Hour() != hour || t.Minute() != min || t.Second() != ss {
		return time.Time{}, fmt.Errorf("invalid date format '%s'", stamp)
	}
	return t, nil
}

func splitSeconds(stamp string) (string, int, error) {
	idx := strings.LastIndex(stamp, ".")
	if idx < 0 {
		return stamp, 0, nil
	}
	secStr := stamp[idx+1:]
	if len(secStr) != 2 {
		return "", 0, fmt.Errorf("invalid date format '%s'", stamp)
	}
	ss, err := strconv.Atoi(secStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid date format '%s'", stamp)
	}
	return stamp[:idx], ss, nil
}

func parseDateFields(base, stamp string) (int, int, int, int, int, error) {
	fail := func() (int, int, int, int, int, error) {
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format '%s'", stamp)
	}
	switch len(base) {
	case 8:
		return parseMMDDhhmm(base, time.Now().Year(), stamp)
	case 10:
		yy, err := strconv.Atoi(base[:2])
		if err != nil {
			return fail()
		}
		cc := 20
		if yy >= 69 {
			cc = 19
		}
		return parseMMDDhhmm(base[2:], cc*100+yy, stamp)
	case 12:
		year, err := strconv.Atoi(base[:4])
		if err != nil {
			return fail()
		}
		return parseMMDDhhmm(base[4:], year, stamp)
	default:
		return fail()
	}
}

func parseMMDDhhmm(s string, year int, stamp string) (int, int, int, int, int, error) {
	month, e1 := strconv.Atoi(s[0:2])
	day, e2 := strconv.Atoi(s[2:4])
	hour, e3 := strconv.Atoi(s[4:6])
	min, e4 := strconv.Atoi(s[6:8])
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("invalid date format '%s'", stamp)
	}
	return year, month, day, hour, min, nil
}
