// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: sha224sum [OPTION]... [FILE]...
Print or check SHA224 (224-bit) checksums.

With no FILE, or when FILE is -, read standard input.
  -b, --binary         read in binary mode
  -c, --check          read checksums from the FILEs and check them
      --tag            create a BSD-style checksum
  -t, --text           read in text mode (default)
  -z, --zero           end each output line with NUL, not newline,
                         and disable file name escaping

The following five options are useful only when verifying checksums:
      --quiet          don't print OK for each successfully verified file
      --status         don't output anything, status code shows success
  -w, --warn           warn about improperly formatted checksum lines
      --help           display this help and exit
      --version        output version information and exit
`

const versionText = `sha224sum (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()
	opts, files := parseArgs(os.Args[1:])
	cfg := hashutil.HashConfig{
		Algorithm: "SHA224",
		NewHash:   sha256.New224,
		DigestLen: 28,
	}
	var stdout io.Writer = os.Stdout
	if opts.zero {
		stdout = &nulWriter{w: os.Stdout}
	}
	if opts.check {
		checkOpts := hashutil.CheckOptions{
			Warn:   opts.warn,
			Quiet:  opts.quiet,
			Status: opts.status,
		}
		exitCode := 0
		for _, f := range files {
			ok, err := hashutil.VerifyChecksums(f, cfg, checkOpts, stdout, os.Stderr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "sha224sum: %s\n", err)
				exitCode = 1
			} else if !ok {
				exitCode = 1
			}
		}
		os.Exit(exitCode)
	}
	os.Exit(hashutil.DigestFiles(files, cfg, opts.binary, opts.tag, stdout, os.Stderr))
}

type nulWriter struct {
	w io.Writer
}

func (n *nulWriter) Write(p []byte) (int, error) {
	replaced := bytes.ReplaceAll(p, []byte{'\n'}, []byte{0})
	nn, err := n.w.Write(replaced)
	if nn > len(p) {
		nn = len(p)
	}
	return nn, err
}

type options struct {
	binary bool
	tag    bool
	check  bool
	warn   bool
	quiet  bool
	status bool
	zero   bool
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
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
		return true, 1
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
		return true, 1
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
	case "--zero":
		opts.zero = true
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
		case 'z':
			opts.zero = true
		}
	}
	return 1
}
