// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/base32"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/encutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	opts, file := parseArgs(os.Args[1:])

	rc, err := encutil.OpenInput(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "base32: %s: %s\n", file, err.Error())
		os.Exit(1)
	}
	defer rc.Close()

	if opts.decode {
		err = encutil.Decode(rc, os.Stdout, encutil.DecoderConfig{
			Decode:        decodeBase32,
			IgnoreGarbage: opts.ignoreGarbage,
		})
	} else {
		err = encutil.Encode(rc, os.Stdout, encutil.EncoderConfig{
			Encode:  encodeBase32,
			WrapCol: opts.wrap,
		})
	}
	if err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "base32: %s\n", err.Error())
		os.Exit(1)
	}
}

func encodeBase32(data []byte) string {
	return base32.StdEncoding.EncodeToString(data)
}

func decodeBase32(s string) ([]byte, error) {
	return base32.StdEncoding.DecodeString(s)
}

type options struct {
	decode        bool
	ignoreGarbage bool
	wrap          int
}

func parseArgs(args []string) (options, string) {
	opts := options{wrap: 76}
	var file string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				file = args[i+1]
			}
			break
		}
		if handled, advance := parseLongFlag(args[i:], &opts); handled {
			i += advance
			continue
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			file = arg
			i++
			continue
		}
		i += parseShortFlags(args[i:], &opts)
	}
	return opts, file
}

func parseLongFlag(remaining []string, opts *options) (bool, int) {
	arg := remaining[0]
	if !strings.HasPrefix(arg, "--") {
		return false, 0
	}
	if arg == "--wrap" || strings.HasPrefix(arg, "--wrap=") {
		return parseWrapFlag(remaining, opts)
	}
	switch arg {
	case "--decode":
		opts.decode = true
	case "--ignore-garbage":
		opts.ignoreGarbage = true
	default:
		fmt.Fprintf(os.Stderr, "base32: unrecognized option '%s'\n", arg)
		os.Exit(1)
	}
	return true, 1
}

func parseWrapFlag(remaining []string, opts *options) (bool, int) {
	arg := remaining[0]
	if strings.HasPrefix(arg, "--wrap=") {
		val := arg[len("--wrap="):]
		n, err := strconv.Atoi(val)
		if err != nil {
			fmt.Fprintf(os.Stderr, "base32: invalid wrap size: '%s'\n", val)
			os.Exit(1)
		}
		opts.wrap = n
		return true, 1
	}
	if len(remaining) < 2 {
		fmt.Fprintf(os.Stderr, "base32: option '--wrap' requires an argument\n")
		os.Exit(1)
	}
	n, err := strconv.Atoi(remaining[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "base32: invalid wrap size: '%s'\n", remaining[1])
		os.Exit(1)
	}
	opts.wrap = n
	return true, 2
}

func parseShortFlags(remaining []string, opts *options) int {
	arg := remaining[0]
	flags := arg[1:]
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'd':
			opts.decode = true
		case 'i':
			opts.ignoreGarbage = true
		case 'w':
			val := flags[j+1:]
			if val != "" {
				n, err := strconv.Atoi(val)
				if err != nil {
					fmt.Fprintf(os.Stderr, "base32: invalid wrap size: '%s'\n", val)
					os.Exit(1)
				}
				opts.wrap = n
				return 1
			}
			if len(remaining) < 2 {
				fmt.Fprintf(os.Stderr, "base32: option requires an argument -- 'w'\n")
				os.Exit(1)
			}
			n, err := strconv.Atoi(remaining[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "base32: invalid wrap size: '%s'\n", remaining[1])
				os.Exit(1)
			}
			opts.wrap = n
			return 2
		default:
			fmt.Fprintf(os.Stderr, "base32: invalid option -- '%c'\n", flags[j])
			os.Exit(1)
		}
	}
	return 1
}
