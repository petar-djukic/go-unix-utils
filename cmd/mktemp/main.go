// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd036-mktemp R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3,
// R3.1, R3.2, R3.3, R3.4, R3.5, R3.6.
package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

var errQuiet = errors.New("")

const defaultTemplate = "tmp.XXXXXXXXXX"
const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

type options struct {
	directory bool
	parentDir string
	useTmpdir bool
	suffix    string
	dryRun    bool
	quiet     bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, template, useDefaultDir, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mktemp: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'mktemp --help' for more information.\n")
		os.Exit(1)
	}

	if err := run(opts, template, useDefaultDir); err != nil {
		if err != errQuiet {
			fmt.Fprintf(os.Stderr, "mktemp: %s\n", err)
		}
		os.Exit(1)
	}
}

func parseArgs(args []string) (options, string, bool, error) {
	var opts options
	var templates []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			templates = append(templates, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			consumed, err := parseLongFlag(arg, args[i+1:], &opts)
			if err != nil {
				return opts, "", false, err
			}
			i += consumed
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			consumed, err := parseShortFlags(arg[1:], args[i+1:], &opts)
			if err != nil {
				return opts, "", false, err
			}
			i += 1 + consumed
			continue
		}
		templates = append(templates, arg)
		i++
	}

	switch len(templates) {
	case 0:
		return opts, defaultTemplate, true, nil
	case 1:
		return opts, templates[0], false, nil
	default:
		return opts, "", false, fmt.Errorf("too many templates")
	}
}

func parseLongFlag(flag string, remaining []string, opts *options) (int, error) {
	switch {
	case flag == "--directory":
		opts.directory = true
		return 1, nil
	case flag == "--dry-run":
		opts.dryRun = true
		return 1, nil
	case flag == "--quiet":
		opts.quiet = true
		return 1, nil
	case flag == "--tmpdir":
		opts.useTmpdir = true
		return 1, nil
	case strings.HasPrefix(flag, "--tmpdir="):
		opts.parentDir = flag[len("--tmpdir="):]
		return 1, nil
	case flag == "--suffix":
		if len(remaining) == 0 {
			return 0, fmt.Errorf("option '--suffix' requires an argument")
		}
		opts.suffix = remaining[0]
		return 2, nil
	case strings.HasPrefix(flag, "--suffix="):
		opts.suffix = flag[len("--suffix="):]
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, remaining []string, opts *options) (int, error) {
	consumed := 0
	for j := range len(flags) {
		switch flags[j] {
		case 'd':
			opts.directory = true
		case 'u':
			opts.dryRun = true
		case 'q':
			opts.quiet = true
		case 't':
			opts.useTmpdir = true
		case 'p':
			if rest := flags[j+1:]; rest != "" {
				opts.parentDir = rest
			} else if len(remaining) > consumed {
				opts.parentDir = remaining[consumed]
				consumed++
			} else {
				return 0, fmt.Errorf("option requires an argument -- 'p'")
			}
			return consumed, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return consumed, nil
}

func run(opts options, template string, useDefaultDir bool) error {
	if opts.suffix != "" && strings.Contains(opts.suffix, "/") {
		return fmt.Errorf("invalid suffix '%s', contains directory separator", opts.suffix)
	}

	fullTemplate := resolveTemplate(opts, template, useDefaultDir)

	xCount := countTrailingX(fullTemplate)
	if xCount < 3 {
		return fmt.Errorf("too few X's in template '%s'", fullTemplate)
	}

	prefix := fullTemplate[:len(fullTemplate)-xCount]

	if opts.dryRun {
		suffix, err := randomString(xCount)
		if err != nil {
			return err
		}
		path := prefix + suffix + opts.suffix
		fmt.Println(path)
		fmt.Fprintln(os.Stderr, "mktemp: warning: --dry-run is discouraged")
		return nil
	}

	for range 100 {
		suffix, err := randomString(xCount)
		if err != nil {
			return err
		}
		path := prefix + suffix + opts.suffix
		if opts.directory {
			err = os.Mkdir(path, 0o700)
		} else {
			var fd *os.File
			fd, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err == nil {
				fd.Close()
			}
		}
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			if opts.quiet {
				return errQuiet
			}
			return fmt.Errorf("failed to create %s via template '%s': %s",
				entityKind(opts.directory), fullTemplate, unwrapErr(err))
		}
		fmt.Println(path)
		return nil
	}

	if opts.quiet {
		return errQuiet
	}
	return fmt.Errorf("failed to create %s via template '%s': too many collisions",
		entityKind(opts.directory), fullTemplate)
}

func resolveTemplate(opts options, template string, useDefaultDir bool) string {
	if opts.parentDir != "" {
		return opts.parentDir + "/" + template
	}
	if useDefaultDir || opts.useTmpdir {
		dir := os.Getenv("TMPDIR")
		if dir == "" {
			dir = "/tmp"
		}
		return dir + "/" + template
	}
	return template
}

func entityKind(isDir bool) string {
	if isDir {
		return "directory"
	}
	return "file"
}

func countTrailingX(s string) int {
	count := 0
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != 'X' {
			break
		}
		count++
	}
	return count
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(charset)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = charset[idx.Int64()]
	}
	return string(b), nil
}

func unwrapErr(err error) string {
	pe, ok := err.(*os.PathError)
	if !ok {
		return err.Error()
	}
	return pe.Err.Error()
}
