// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

type options struct {
	noCreate   bool
	accessOnly bool
	modOnly    bool
	stampStr   string
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
	t := time.Now()
	if opts.stampStr != "" {
		var err error
		t, err = parseTimestamp(opts.stampStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "touch: %s\n", err)
			return 1
		}
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

func touchFile(path string, opts options, t time.Time) error {
	_, statErr := os.Stat(path)
	if os.IsNotExist(statErr) {
		if opts.noCreate {
			return nil
		}
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("cannot touch '%s': %s", path, sysMsg(err))
		}
		f.Close()
	}
	atime, mtime, err := resolveTimestamps(path, opts, t)
	if err != nil {
		return err
	}
	if err := os.Chtimes(path, atime, mtime); err != nil {
		return fmt.Errorf("cannot touch '%s': %s", path, sysMsg(err))
	}
	return nil
}

func resolveTimestamps(path string, opts options, t time.Time) (time.Time, time.Time, error) {
	if !opts.accessOnly && !opts.modOnly {
		return t, t, nil
	}
	info, err := sys.Stat(path)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("cannot touch '%s': %s", path, sysMsg(err))
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
			if err := parseLongFlag(arg, &opts); err != nil {
				return options{}, nil, err
			}
			i++
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

func parseLongFlag(flag string, opts *options) error {
	switch flag {
	case "--no-create":
		opts.noCreate = true
	default:
		return fmt.Errorf("unrecognized option '%s'", flag)
	}
	return nil
}

func parseShortFlags(flags string, args []string, argIdx int, opts *options) (int, error) {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'a':
			opts.accessOnly = true
		case 'c':
			opts.noCreate = true
		case 'm':
			opts.modOnly = true
		case 't':
			return handleStampFlag(flags[j+1:], args, argIdx, opts)
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return 0, nil
}

func handleStampFlag(remainder string, args []string, argIdx int, opts *options) (int, error) {
	stamp := remainder
	extra := 0
	if stamp == "" {
		if argIdx+1 >= len(args) {
			return 0, fmt.Errorf("option requires an argument -- 't'")
		}
		stamp = args[argIdx+1]
		extra = 1
	}
	opts.stampStr = stamp
	return extra, nil
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
