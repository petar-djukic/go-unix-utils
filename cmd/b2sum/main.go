// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"hash"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/blake2b"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func newBLAKE2b(size int) func() hash.Hash {
	return func() hash.Hash {
		h, _ := blake2b.New(size, nil)
		return h
	}
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, files := parseArgs(os.Args[1:])

	if opts.length == 0 {
		opts.length = 512
	}
	if opts.length <= 0 || opts.length%8 != 0 || opts.length > 512 {
		fmt.Fprintf(os.Stderr, "b2sum: invalid length: %d\n", opts.length)
		os.Exit(1)
	}

	digestBytes := opts.length / 8
	algorithm := "BLAKE2b"
	if opts.length != 512 {
		algorithm = fmt.Sprintf("BLAKE2b-%d", opts.length)
	}

	cfg := hashutil.HashConfig{
		Algorithm: algorithm,
		NewHash:   newBLAKE2b(digestBytes),
		DigestLen: digestBytes,
	}
	if opts.check {
		checkOpts := hashutil.CheckOptions{
			Warn:   opts.warn,
			Quiet:  opts.quiet,
			Status: opts.status,
		}
		exitCode := 0
		for _, f := range files {
			ok, err := hashutil.VerifyChecksums(f, cfg, checkOpts, os.Stdout, os.Stderr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "b2sum: %s\n", err)
				exitCode = 1
			} else if !ok {
				exitCode = 1
			}
		}
		os.Exit(exitCode)
	}
	os.Exit(hashutil.DigestFiles(files, cfg, opts.binary, opts.tag, os.Stdout, os.Stderr))
}

type options struct {
	binary bool
	tag    bool
	check  bool
	warn   bool
	quiet  bool
	status bool
	length int
}

func parseArgs(args []string) (options, []string) {
	var opts options
	var files []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if handled, advance := parseLongFlag(args[i:], &opts); handled {
			i += advance
			continue
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			i++
			continue
		}
		i += parseShortFlags(arg[1:], &opts)
	}
	return opts, files
}

func parseLongFlag(remaining []string, opts *options) (bool, int) {
	arg := remaining[0]
	if !strings.HasPrefix(arg, "--") {
		return false, 0
	}
	if arg == "--length" || strings.HasPrefix(arg, "--length=") {
		return parseLengthFlag(remaining, opts)
	}
	switch arg {
	case "--binary":
		opts.binary = true
	case "--text":
		opts.binary = false
	case "--tag":
		opts.tag = true
	case "--check":
		opts.check = true
	case "--warn":
		opts.warn = true
	case "--quiet":
		opts.quiet = true
	case "--status":
		opts.status = true
	default:
		return false, 0
	}
	return true, 1
}

func parseLengthFlag(remaining []string, opts *options) (bool, int) {
	arg := remaining[0]
	if strings.HasPrefix(arg, "--length=") {
		val := arg[len("--length="):]
		n, err := strconv.Atoi(val)
		if err != nil {
			fmt.Fprintf(os.Stderr, "b2sum: invalid length: '%s'\n", val)
			os.Exit(1)
		}
		opts.length = n
		return true, 1
	}
	if len(remaining) < 2 {
		fmt.Fprintf(os.Stderr, "b2sum: option '--length' requires an argument\n")
		os.Exit(1)
	}
	n, err := strconv.Atoi(remaining[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "b2sum: invalid length: '%s'\n", remaining[1])
		os.Exit(1)
	}
	opts.length = n
	return true, 2
}

func parseShortFlags(flags string, opts *options) int {
	for _, ch := range flags {
		switch ch {
		case 'b':
			opts.binary = true
		case 't':
			opts.binary = false
		case 'c':
			opts.check = true
		case 'w':
			opts.warn = true
		}
	}
	return 1
}
