// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd049-realpath R1.1-R1.5, R2.1-R2.3, R3.1-R3.3, R4.1-R4.3.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: realpath [OPTION]... FILE...
Print the resolved absolute file name;
all but the last component must exist

  -e, --canonicalize-existing  all components of the path must exist
  -m, --canonicalize-missing   no path components need exist or be a directory
  -s, --strip, --no-symlinks   don't expand symlinks
      --relative-to=DIR        print the resolved path relative to DIR
      --relative-base=DIR      print absolute paths unless paths below DIR
      --help                   display this help and exit
      --version                output version information and exit
`

const versionText = `realpath (go-unix-utils) dev
`

type resolveMode int

const (
	modeDefault resolveMode = iota
	modeCanonE
	modeCanonM
)

type config struct {
	mode         resolveMode
	strip        bool
	relativeTo   string
	relativeBase string
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, operands := parseArgs(os.Args[1:])
	if len(operands) == 0 {
		fmt.Fprintln(os.Stderr, "realpath: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'realpath --help' for more information.")
		os.Exit(1)
	}
	relTo, relBase := resolveRelDirs(cfg)
	exitCode := 0
	for _, op := range operands {
		result, err := resolve(cfg, op)
		if err != nil {
			fmt.Fprintf(os.Stderr, "realpath: %s: %s\n", op, sysError(err))
			exitCode = 1
			continue
		}
		result = applyRelative(result, relTo, relBase)
		fmt.Fprintln(os.Stdout, result)
	}
	os.Exit(exitCode)
}

func resolveRelDirs(cfg config) (string, string) {
	var relTo, relBase string
	if cfg.relativeTo != "" {
		r, err := resolve(cfg, cfg.relativeTo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "realpath: %s: %s\n", cfg.relativeTo, sysError(err))
			os.Exit(1)
		}
		relTo = r
	}
	if cfg.relativeBase != "" {
		r, err := resolve(cfg, cfg.relativeBase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "realpath: %s: %s\n", cfg.relativeBase, sysError(err))
			os.Exit(1)
		}
		relBase = r
	}
	return relTo, relBase
}

func parseArgs(args []string) (config, []string) {
	var cfg config
	var operands []string
	pastDashDash := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if pastDashDash {
			operands = append(operands, arg)
			continue
		}
		switch {
		case arg == "--":
			pastDashDash = true
		case arg == "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case arg == "--canonicalize-existing":
			cfg.mode = modeCanonE
		case arg == "--canonicalize-missing":
			cfg.mode = modeCanonM
		case arg == "--strip", arg == "--no-symlinks":
			cfg.strip = true
		case arg == "--relative-to" || strings.HasPrefix(arg, "--relative-to="):
			cfg.relativeTo, i = parseLongValue(args, arg, "--relative-to", i)
		case arg == "--relative-base" || strings.HasPrefix(arg, "--relative-base="):
			cfg.relativeBase, i = parseLongValue(args, arg, "--relative-base", i)
		case strings.HasPrefix(arg, "--"):
			exitUnknownOpt(arg)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			cfg.mode, cfg.strip = parseShortFlags(arg[1:], cfg.mode, cfg.strip)
		default:
			operands = append(operands, arg)
		}
	}
	return cfg, operands
}

func parseLongValue(args []string, arg, prefix string, i int) (string, int) {
	if arg == prefix {
		if i+1 >= len(args) {
			fmt.Fprintf(os.Stderr, "realpath: option '%s' requires an argument\n", prefix)
			fmt.Fprintln(os.Stderr, "Try 'realpath --help' for more information.")
			os.Exit(1)
		}
		return args[i+1], i + 1
	}
	return arg[len(prefix)+1:], i
}

func exitUnknownOpt(arg string) {
	fmt.Fprintf(os.Stderr, "realpath: unrecognized option '%s'\n", arg)
	fmt.Fprintln(os.Stderr, "Try 'realpath --help' for more information.")
	os.Exit(1)
}

func parseShortFlags(flags string, m resolveMode, strip bool) (resolveMode, bool) {
	for _, ch := range flags {
		switch ch {
		case 'e':
			m = modeCanonE
		case 'm':
			m = modeCanonM
		case 's':
			strip = true
		default:
			fmt.Fprintf(os.Stderr, "realpath: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'realpath --help' for more information.")
			os.Exit(1)
		}
	}
	return m, strip
}

func resolve(cfg config, path string) (string, error) {
	if cfg.strip {
		return resolveStrip(cfg.mode, path)
	}
	switch cfg.mode {
	case modeCanonE:
		return canonicalizeE(path)
	case modeCanonM:
		return canonicalizeM(path)
	default:
		return canonicalizeDefault(path)
	}
}

func resolveStrip(m resolveMode, path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	cleaned := filepath.Clean(absPath)
	if m == modeCanonE {
		if _, err := os.Stat(cleaned); err != nil {
			return "", err
		}
	}
	return cleaned, nil
}

func applyRelative(result, relativeTo, relativeBase string) string {
	if relativeTo == "" && relativeBase == "" {
		return result
	}
	if relativeBase != "" && !hasPathPrefix(result, relativeBase) {
		return result
	}
	dir := relativeTo
	if dir == "" {
		dir = relativeBase
	}
	rel, err := filepath.Rel(dir, result)
	if err != nil {
		return result
	}
	return rel
}

func hasPathPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+string(filepath.Separator))
}

func canonicalizeDefault(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return resolved, nil
	}
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

func canonicalizeE(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absPath)
}

func canonicalizeM(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return doCanonM(absPath)
}

func doCanonM(absPath string) (string, error) {
	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return resolved, nil
	}
	if absPath == "/" {
		return "/", nil
	}
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)
	resolvedDir, err := doCanonM(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

func sysError(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		msg := pe.Err.Error()
		if len(msg) > 0 {
			return strings.ToUpper(msg[:1]) + msg[1:]
		}
		return msg
	}
	return err.Error()
}
