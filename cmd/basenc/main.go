// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/encutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const z85Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.-:+=^!/*?&<>()[]{}@%$#"

var z85Decode [256]byte

func init() {
	for i := range z85Decode {
		z85Decode[i] = 0xFF
	}
	for i := range len(z85Alphabet) {
		z85Decode[z85Alphabet[i]] = byte(i)
	}
}

var encodingFlags = map[string]string{
	"--base64":    "base64",
	"--base64url": "base64url",
	"--base32":    "base32",
	"--base32hex": "base32hex",
	"--base16":    "base16",
	"--z85":       "z85",
}

type options struct {
	encoding      string
	decode        bool
	ignoreGarbage bool
	wrap          int
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, file := parseArgs(os.Args[1:])

	if opts.encoding == "" {
		fmt.Fprintf(os.Stderr, "basenc: missing encoding type\n")
		os.Exit(1)
	}

	rc, err := encutil.OpenInput(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "basenc: %s: %s\n", file, err.Error())
		os.Exit(1)
	}
	defer rc.Close()

	if opts.decode {
		err = encutil.Decode(rc, os.Stdout, encutil.DecoderConfig{
			Decode:        decoderFor(opts.encoding),
			IgnoreGarbage: opts.ignoreGarbage,
		})
	} else {
		err = encutil.Encode(rc, os.Stdout, encutil.EncoderConfig{
			Encode:  encoderFor(opts.encoding),
			WrapCol: opts.wrap,
		})
	}
	if err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "basenc: %s\n", err.Error())
		os.Exit(1)
	}
}

func encoderFor(encoding string) func([]byte) string {
	switch encoding {
	case "base64":
		return func(d []byte) string { return base64.StdEncoding.EncodeToString(d) }
	case "base64url":
		return func(d []byte) string { return base64.URLEncoding.EncodeToString(d) }
	case "base32":
		return func(d []byte) string { return base32.StdEncoding.EncodeToString(d) }
	case "base32hex":
		return func(d []byte) string { return base32.HexEncoding.EncodeToString(d) }
	case "base16":
		return func(d []byte) string { return strings.ToUpper(hex.EncodeToString(d)) }
	default:
		return encodeZ85
	}
}

func decoderFor(encoding string) func(string) ([]byte, error) {
	switch encoding {
	case "base64":
		return base64.StdEncoding.DecodeString
	case "base64url":
		return base64.URLEncoding.DecodeString
	case "base32":
		return base32.StdEncoding.DecodeString
	case "base32hex":
		return base32.HexEncoding.DecodeString
	case "base16":
		return hex.DecodeString
	default:
		return decodeZ85
	}
}

func encodeZ85(data []byte) string {
	if len(data)%4 != 0 {
		fmt.Fprintf(os.Stderr, "basenc: invalid input (length must be multiple of 4)\n")
		os.Exit(1)
	}
	var buf strings.Builder
	buf.Grow(len(data) * 5 / 4)
	var chars [5]byte
	for i := 0; i < len(data); i += 4 {
		val := uint32(data[i])<<24 | uint32(data[i+1])<<16 | uint32(data[i+2])<<8 | uint32(data[i+3])
		for j := 4; j >= 0; j-- {
			chars[j] = z85Alphabet[val%85]
			val /= 85
		}
		buf.Write(chars[:])
	}
	return buf.String()
}

func decodeZ85(s string) ([]byte, error) {
	if len(s)%5 != 0 {
		return nil, fmt.Errorf("invalid input")
	}
	out := make([]byte, len(s)*4/5)
	for i := 0; i < len(s); i += 5 {
		var val uint32
		for j := range 5 {
			v := z85Decode[s[i+j]]
			if v == 0xFF {
				return nil, fmt.Errorf("invalid input")
			}
			val = val*85 + uint32(v)
		}
		k := i / 5 * 4
		out[k] = byte(val >> 24)
		out[k+1] = byte(val >> 16)
		out[k+2] = byte(val >> 8)
		out[k+3] = byte(val)
	}
	return out, nil
}

func parseArgs(args []string) (options, string) {
	opts := options{wrap: 76}
	var file string
	fileSet := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			for _, extra := range args[i+1:] {
				if fileSet {
					fmt.Fprintf(os.Stderr, "basenc: extra operand '%s'\n", extra)
					os.Exit(1)
				}
				file = extra
				fileSet = true
			}
			break
		}
		if handled, advance := parseLongFlag(args[i:], &opts); handled {
			i += advance
			continue
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			if fileSet {
				fmt.Fprintf(os.Stderr, "basenc: extra operand '%s'\n", arg)
				os.Exit(1)
			}
			file = arg
			fileSet = true
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
	if enc, ok := encodingFlags[arg]; ok {
		if opts.encoding != "" {
			fmt.Fprintf(os.Stderr, "basenc: extra operand '%s'\n", arg)
			os.Exit(1)
		}
		opts.encoding = enc
		return true, 1
	}
	switch arg {
	case "--decode":
		opts.decode = true
	case "--ignore-garbage":
		opts.ignoreGarbage = true
	default:
		fmt.Fprintf(os.Stderr, "basenc: unrecognized option '%s'\n", arg)
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
			fmt.Fprintf(os.Stderr, "basenc: invalid wrap size: '%s'\n", val)
			os.Exit(1)
		}
		opts.wrap = n
		return true, 1
	}
	if len(remaining) < 2 {
		fmt.Fprintf(os.Stderr, "basenc: option '--wrap' requires an argument\n")
		os.Exit(1)
	}
	n, err := strconv.Atoi(remaining[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "basenc: invalid wrap size: '%s'\n", remaining[1])
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
					fmt.Fprintf(os.Stderr, "basenc: invalid wrap size: '%s'\n", val)
					os.Exit(1)
				}
				opts.wrap = n
				return 1
			}
			if len(remaining) < 2 {
				fmt.Fprintf(os.Stderr, "basenc: option requires an argument -- 'w'\n")
				os.Exit(1)
			}
			n, err := strconv.Atoi(remaining[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "basenc: invalid wrap size: '%s'\n", remaining[1])
				os.Exit(1)
			}
			opts.wrap = n
			return 2
		default:
			fmt.Fprintf(os.Stderr, "basenc: invalid option -- '%c'\n", flags[j])
			os.Exit(1)
		}
	}
	return 1
}
