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
	wrap, decode, ignoreGarbage, file := parseArgs(os.Args[1:])

	rc, err := encutil.OpenInput(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "base64: %s: %s\n", file, err.Error())
		os.Exit(1)
	}
	defer rc.Close()

	if decode {
		err = encutil.Decode(rc, os.Stdout, encutil.DecoderConfig{
			Decode:        decodeBase64,
			IgnoreGarbage: ignoreGarbage,
		})
	} else {
		err = encutil.Encode(rc, os.Stdout, encutil.EncoderConfig{
			Encode:  encodeBase64,
			WrapCol: wrap,
		})
	}
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

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func parseArgs(args []string) (int, bool, bool, string) {
	wrap := 76
	var decode, ignoreGarbage bool
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
		if handled, advance := parseLongFlag(args[i:], &wrap, &decode, &ignoreGarbage); handled {
			i += advance
			continue
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			file = arg
			i++
			continue
		}
		i += parseShortFlags(args[i:], &wrap, &decode, &ignoreGarbage)
	}
	return wrap, decode, ignoreGarbage, file
}

func parseLongFlag(remaining []string, wrap *int, decode, ignoreGarbage *bool) (bool, int) {
	arg := remaining[0]
	if !strings.HasPrefix(arg, "--") {
		return false, 0
	}
	if arg == "--decode" {
		*decode = true
		return true, 1
	}
	if arg == "--ignore-garbage" {
		*ignoreGarbage = true
		return true, 1
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

func parseShortFlags(remaining []string, wrap *int, decode, ignoreGarbage *bool) int {
	arg := remaining[0]
	flags := arg[1:]
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'd':
			*decode = true
		case 'i':
			*ignoreGarbage = true
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
