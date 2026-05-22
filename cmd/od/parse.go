// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strings"
)

func parseArgs(args []string) (options, []string, error) {
	var opts options
	opts.addrRadix = 'o'
	var files []string
	endOfFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || arg == "" || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if arg == "-" {
			files = append(files, arg)
			continue
		}
		consumed, err := parseLongOrShort(arg, args[i:], &opts)
		if err != nil {
			return opts, nil, err
		}
		i += consumed - 1
	}
	return opts, files, nil
}

func parseLongOrShort(arg string, remaining []string, opts *options) (int, error) {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(arg, opts)
	}
	return parseShortFlags(arg[1:], remaining, opts)
}

func parseLongFlag(arg string, opts *options) (int, error) {
	switch {
	case strings.HasPrefix(arg, "--format="):
		return 1, addTypeString(opts, arg[len("--format="):])
	case strings.HasPrefix(arg, "--address-radix="):
		return 1, setAddrRadix(opts, arg[len("--address-radix="):])
	case arg == "--output-duplicates":
		opts.verbose = true
		return 1, nil
	case strings.HasPrefix(arg, "--skip-bytes="):
		return 1, parseSkipBytes(opts, arg[len("--skip-bytes="):])
	case strings.HasPrefix(arg, "--read-bytes="):
		return 1, parseReadBytes(opts, arg[len("--read-bytes="):])
	case strings.HasPrefix(arg, "--width="):
		return 1, parseWidth(opts, arg[len("--width="):])
	case arg == "--width":
		opts.outputWidth = 32
		return 1, nil
	}
	return 1, fmt.Errorf("unrecognized option '%s'", arg)
}

func parseShortFlags(flags string, remaining []string, opts *options) (int, error) {
	consumed := 1
	for j := 0; j < len(flags); j++ {
		rest := flags[j+1:]
		switch flags[j] {
		case 't':
			val, extra := flagValue(rest, remaining, consumed)
			return consumed + extra, addTypeString(opts, val)
		case 'A':
			val, extra := flagValue(rest, remaining, consumed)
			return consumed + extra, setAddrRadix(opts, val)
		case 'j':
			val, extra := flagValue(rest, remaining, consumed)
			return consumed + extra, parseSkipBytes(opts, val)
		case 'N':
			val, extra := flagValue(rest, remaining, consumed)
			return consumed + extra, parseReadBytes(opts, val)
		case 'v':
			opts.verbose = true
		case 'w':
			extra, err := parseShortWidth(rest, opts)
			return consumed + extra, err
		default:
			if ts, ok := traditionalFlag(flags[j]); ok {
				opts.types = append(opts.types, ts)
			} else {
				return consumed, fmt.Errorf("invalid option -- '%c'", flags[j])
			}
		}
	}
	return consumed, nil
}

func traditionalFlag(c byte) (typeSpec, bool) {
	switch c {
	case 'b':
		return typeSpec{format: 'o', size: 1}, true
	case 'c':
		return typeSpec{format: 'c', size: 1}, true
	case 'd':
		return typeSpec{format: 'u', size: 2}, true
	case 'o':
		return typeSpec{format: 'o', size: 2}, true
	case 's':
		return typeSpec{format: 'd', size: 2}, true
	case 'x':
		return typeSpec{format: 'x', size: 2}, true
	}
	return typeSpec{}, false
}

func parseShortWidth(rest string, opts *options) (int, error) {
	if len(rest) > 0 {
		return 0, parseWidth(opts, rest)
	}
	opts.outputWidth = 32
	return 0, nil
}

func flagValue(rest string, remaining []string, consumed int) (string, int) {
	if len(rest) > 0 {
		return rest, 0
	}
	if consumed < len(remaining) {
		return remaining[consumed], 1
	}
	return "", 0
}

func addTypeString(opts *options, s string) error {
	ts, err := parseTypeString(s)
	if err != nil {
		return err
	}
	opts.types = append(opts.types, ts...)
	return nil
}

func setAddrRadix(opts *options, val string) error {
	switch val {
	case "d", "o", "x", "n":
		opts.addrRadix = val[0]
		return nil
	}
	return fmt.Errorf("invalid address radix '%s'; must be one of [doxn]", val)
}

func parseSkipBytes(opts *options, val string) error {
	n, err := parseByteCount(val)
	if err != nil {
		return err
	}
	opts.skipBytes = n
	return nil
}

func parseReadBytes(opts *options, val string) error {
	n, err := parseByteCount(val)
	if err != nil {
		return err
	}
	opts.readBytes = n
	opts.readLimit = true
	return nil
}

func parseWidth(opts *options, val string) error {
	n := 0
	for _, c := range val {
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid width: '%s'", val)
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		n = 16
	}
	opts.outputWidth = n
	return nil
}

func parseByteCount(s string) (int64, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("invalid byte count: ''")
	}
	mult := int64(1)
	num := s
	switch s[len(s)-1] {
	case 'b':
		mult = 512
		num = s[:len(s)-1]
	case 'k':
		mult = 1024
		num = s[:len(s)-1]
	case 'm':
		mult = 1048576
		num = s[:len(s)-1]
	}
	n := int64(0)
	for _, c := range num {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid byte count: '%s'", s)
		}
		n = n*10 + int64(c-'0')
	}
	return n * mult, nil
}

func parseTypeString(s string) ([]typeSpec, error) {
	var specs []typeSpec
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case 'a', 'c':
			specs = append(specs, typeSpec{format: c, size: 1})
		case 'd', 'f', 'o', 'u', 'x':
			size, advance := parseSize(s[i+1:], c)
			specs = append(specs, typeSpec{format: c, size: size})
			i += advance
		default:
			return nil, fmt.Errorf("invalid type string '%s'", s)
		}
	}
	return specs, nil
}

func parseSize(rest string, format byte) (int, int) {
	if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
		n, j := 0, 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			n = n*10 + int(rest[j]-'0')
			j++
		}
		if n == 0 {
			n = defaultSize(format)
		}
		return n, j
	}
	if len(rest) > 0 {
		switch rest[0] {
		case 'C':
			return 1, 1
		case 'S':
			return 2, 1
		case 'I':
			return 4, 1
		case 'L':
			return 8, 1
		}
	}
	return defaultSize(format), 0
}

func defaultSize(format byte) int {
	if format == 'f' {
		return 8
	}
	return 4
}
