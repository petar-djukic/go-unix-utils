// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/shred implements srd099-shred R1.1-R1.4, R2.1-R2.4, R3.1-R3.3.
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: shred [OPTION]... FILE...
Overwrite the specified FILE(s) repeatedly, in order to make it harder
for even very expensive hardware probing to recover the data.

If FILE is -, shred standard output.

Mandatory arguments to long options are mandatory for short options too.
  -n, --iterations=N  overwrite N times instead of the default (3)
  -u, --remove        truncate and remove file after overwriting
  -v, --verbose        show progress
  -z, --zero           add a final overwrite with zeros to hide shredding
  -s, --size=N         shred this many bytes (suffixes like K, M, G accepted)
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `shred (go-unix-utils) dev
`

type options struct {
	iterations int
	remove     bool
	verbose    bool
	zero       bool
	size       int64
	sizeSet    bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "shred: missing file operand")
		fmt.Fprintln(os.Stderr, "Try 'shred --help' for more information.")
		os.Exit(1)
	}

	exitCode := 0
	for _, file := range files {
		if err := shredFile(opts, file); err != nil {
			fmt.Fprintf(os.Stderr, "shred: %s: %s\n", file, err)
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func parseArgs(args []string) (options, []string) {
	var opts options
	var sizeStr string

	fs := flag.NewFlagSet("shred", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&opts.iterations, "n", 3, "")
	fs.IntVar(&opts.iterations, "iterations", 3, "")
	fs.BoolVar(&opts.remove, "u", false, "")
	fs.BoolVar(&opts.remove, "remove", false, "")
	fs.BoolVar(&opts.verbose, "v", false, "")
	fs.BoolVar(&opts.verbose, "verbose", false, "")
	fs.BoolVar(&opts.zero, "z", false, "")
	fs.BoolVar(&opts.zero, "zero", false, "")
	fs.StringVar(&sizeStr, "s", "", "")
	fs.StringVar(&sizeStr, "size", "", "")

	help := fs.Bool("help", false, "")
	version := fs.Bool("version", false, "")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "shred: %s\n", err)
		fmt.Fprintln(os.Stderr, "Try 'shred --help' for more information.")
		os.Exit(1)
	}

	if *help {
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
	}
	if *version {
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
	}

	if sizeStr != "" {
		n, err := sizeparse.Parse(sizeStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "shred: invalid size: '%s'\n", sizeStr)
			os.Exit(1)
		}
		opts.size = n
		opts.sizeSet = true
	}

	return opts, fs.Args()
}

func shredFile(opts options, path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	size, err := fileSize(f, opts)
	if err != nil {
		return err
	}

	for i := 0; i < opts.iterations; i++ {
		if opts.verbose {
			printPass(path, i+1, opts)
		}
		if err := overwritePass(f, size, rand.Reader); err != nil {
			return err
		}
	}

	if opts.zero {
		if opts.verbose {
			printZeroPass(path, opts)
		}
		if err := zeroPass(f, size); err != nil {
			return err
		}
	}

	if err := f.Sync(); err != nil {
		return err
	}
	f.Close()

	if opts.remove {
		return removeFile(path)
	}
	return nil
}

func fileSize(f *os.File, opts options) (int64, error) {
	if opts.sizeSet {
		return opts.size, nil
	}
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func overwritePass(f *os.File, size int64, src io.Reader) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err := io.CopyN(f, src, size)
	return err
}

func zeroPass(f *os.File, size int64) error {
	return overwritePass(f, size, zeroReader{})
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func removeFile(path string) error {
	if err := os.Truncate(path, 0); err != nil {
		return err
	}
	return os.Remove(path)
}

func printPass(path string, pass int, opts options) {
	total := opts.iterations
	if opts.zero {
		total++
	}
	fmt.Fprintf(os.Stderr, "shred: %s: pass %d/%d (random)...\n",
		path, pass, total)
}

func printZeroPass(path string, opts options) {
	total := opts.iterations
	if opts.zero {
		total++
	}
	fmt.Fprintf(os.Stderr, "shred: %s: pass %d/%d (000000)...\n",
		path, total, total)
}
