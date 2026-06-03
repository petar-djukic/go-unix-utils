// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"crypto/sha1"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	opts, files := parseArgs(os.Args[1:])
	cfg := hashutil.HashConfig{
		Algorithm: "SHA1",
		NewHash:   sha1.New,
		DigestLen: 20,
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
				fmt.Fprintf(os.Stderr, "sha1sum: %s\n", err)
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
		if handled, advance := parseLongFlag(arg, &opts); handled {
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

func parseLongFlag(arg string, opts *options) (bool, int) {
	if !strings.HasPrefix(arg, "--") {
		return false, 0
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
