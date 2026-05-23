// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/base64"
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
	wrap, file := parseArgs(os.Args[1:])

	rc, err := encutil.OpenInput(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "base64: %s: %s\n", file, err.Error())
		os.Exit(1)
	}
	defer rc.Close()

	err = encutil.Encode(rc, os.Stdout, encutil.EncoderConfig{
		Encode:  encodeBase64,
		WrapCol: wrap,
	})
	if err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "base64: %s\n", err.Error())
		os.Exit(1)
	}
}

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func parseArgs(args []string) (int, string) {
	wrap := 76
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
		if handled, advance := parseLongFlag(args[i:], &wrap); handled {
			i += advance
			continue
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			file = arg
			i++
			continue
		}
		i += parseShortFlags(args[i:], &wrap)
	}
	return wrap, file
}

func parseLongFlag(remaining []string, wrap *int) (bool, int) {
	arg := remaining[0]
	if !strings.HasPrefix(arg, "--") {
		return false, 0
	}
	if arg == "--wrap" || strings.HasPrefix(arg, "--wrap=") {
		return parseWrapFlag(remaining, wrap)
	}
	fmt.Fprintf(os.Stderr, "base64: unrecognized option '%s'\n", arg)
	os.Exit(1)
	return false, 0
}

func parseWrapFlag(remaining []string, wrap *int) (bool, int) {
	arg := remaining[0]
	if strings.HasPrefix(arg, "--wrap=") {
		val := arg[len("--wrap="):]
		n, err := strconv.Atoi(val)
		if err != nil {
			fmt.Fprintf(os.Stderr, "base64: invalid wrap size: '%s'\n", val)
			os.Exit(1)
		}
		*wrap = n
		return true, 1
	}
	if len(remaining) < 2 {
		fmt.Fprintf(os.Stderr, "base64: option '--wrap' requires an argument\n")
		os.Exit(1)
	}
	n, err := strconv.Atoi(remaining[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "base64: invalid wrap size: '%s'\n", remaining[1])
		os.Exit(1)
	}
	*wrap = n
	return true, 2
}

func parseShortFlags(remaining []string, wrap *int) int {
	arg := remaining[0]
	flags := arg[1:]
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'w':
			val := flags[j+1:]
			if val != "" {
				n, err := strconv.Atoi(val)
				if err != nil {
					fmt.Fprintf(os.Stderr, "base64: invalid wrap size: '%s'\n", val)
					os.Exit(1)
				}
				*wrap = n
				return 1
			}
			if len(remaining) < 2 {
				fmt.Fprintf(os.Stderr, "base64: option requires an argument -- 'w'\n")
				os.Exit(1)
			}
			n, err := strconv.Atoi(remaining[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "base64: invalid wrap size: '%s'\n", remaining[1])
				os.Exit(1)
			}
			*wrap = n
			return 2
		default:
			fmt.Fprintf(os.Stderr, "base64: invalid option -- '%c'\n", flags[j])
			os.Exit(1)
		}
	}
	return 1
}
