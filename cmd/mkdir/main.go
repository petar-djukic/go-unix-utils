// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd034-mkdir R1.1-R1.4, R2.1-R2.3, R3.1-R3.4:
// cmd/mkdir creates directories with optional parent creation (-p),
// explicit permission mode setting (-m), verbose output (-v),
// and SELinux context flag (-Z/--context, silently accepted on non-SELinux).
// Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error and verbose messages to match GNU mkdir format.
const progName = "mkdir"

// who bitmask constants for symbolic mode parsing.
const (
	whoUser  = 1 << iota // u
	whoGroup             // g
	whoOther             // o
)

// mkdirOptions holds the parsed flags for a mkdir invocation.
type mkdirOptions struct {
	parents     bool   // -p: create parent directories as needed
	mode        string // -m: permission mode (octal or symbolic)
	verbose     bool   // -v: print message for each directory created
	context     bool   // -Z/--context: SELinux context (silently accepted on non-SELinux)
	showVersion bool
	showHelp    bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, dirs := parseArgs(os.Args[1:])

	if opts.showVersion {
		fmt.Println("mkdir (go-unix-utils) 0.1")
		os.Exit(0)
	}

	if opts.showHelp {
		printUsage(os.Stdout)
		os.Exit(0)
	}

	if len(dirs) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	// R3.1, R3.2: parse mode once if specified.
	var dirMode os.FileMode = 0o777
	modeSpecified := false
	if opts.mode != "" {
		m, err := parseMode(opts.mode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: invalid mode '%s'\n", progName, opts.mode)
			os.Exit(1)
		}
		dirMode = m
		modeSpecified = true
	}

	exitCode := 0
	for _, dir := range dirs {
		var err error
		if opts.parents {
			err = makeDirParents(dir, dirMode, modeSpecified, opts.verbose)
		} else {
			err = makeDir(dir, dirMode, modeSpecified, opts.verbose)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// parseArgs separates flags from directory arguments. Flags are processed
// until the first non-flag argument or "--".
func parseArgs(args []string) (*mkdirOptions, []string) {
	opts := &mkdirOptions{}
	var dirs []string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone {
			dirs = append(dirs, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--parents":
				opts.parents = true
			case arg == "--verbose":
				opts.verbose = true
			case arg == "--context":
				// R3.3: silently accepted on non-SELinux systems.
				opts.context = true
			case strings.HasPrefix(arg, "--context="):
				// R3.3: --context=CTX form silently accepted.
				opts.context = true
			case arg == "--version":
				opts.showVersion = true
			case arg == "--help":
				opts.showHelp = true
			case arg == "--mode":
				if i+1 < len(args) {
					i++
					opts.mode = args[i]
				}
			case strings.HasPrefix(arg, "--mode="):
				opts.mode = arg[len("--mode="):]
			}
			continue
		}
		// Short options.
		if arg[0] == '-' && len(arg) > 1 {
			for j := 1; j < len(arg); j++ {
				switch arg[j] {
				case 'p':
					opts.parents = true
				case 'v':
					opts.verbose = true
				case 'Z':
					// R3.3: silently accepted on non-SELinux systems.
					opts.context = true
				case 'm':
					// -m consumes the rest of the arg or the next arg.
					rest := arg[j+1:]
					if rest != "" {
						opts.mode = rest
					} else if i+1 < len(args) {
						i++
						opts.mode = args[i]
					}
					j = len(arg) // stop processing this arg
				}
			}
			continue
		}
		dirs = append(dirs, arg)
	}
	return opts, dirs
}

// makeDir creates a single directory without parent creation.
// R1.1, R1.3, R1.4.
func makeDir(dir string, mode os.FileMode, modeSpecified, verbose bool) error {
	if err := os.Mkdir(dir, 0o777); err != nil {
		return fmt.Errorf("cannot create directory '%s': %v", dir, unwrapPathError(err))
	}
	// R3.1: apply exact mode via chmod since os.Mkdir applies umask.
	if modeSpecified {
		if err := os.Chmod(dir, mode); err != nil {
			return fmt.Errorf("setting permissions of '%s': %v", dir, unwrapPathError(err))
		}
	}
	if verbose {
		// R3.4: GNU mkdir verbose output goes to stdout.
		fmt.Fprintf(os.Stdout, "%s: created directory '%s'\n", progName, dir)
	}
	return nil
}

// makeDirParents creates a directory and any missing parent directories.
// R2.1, R2.2, R2.3, R3.3.
func makeDirParents(dir string, mode os.FileMode, modeSpecified, verbose bool) error {
	cleaned := filepath.Clean(dir)

	// Walk up from target to find the deepest existing ancestor.
	var needCreate []string
	cur := cleaned
	for {
		fi, err := os.Stat(cur)
		if err == nil {
			if !fi.IsDir() {
				return fmt.Errorf("cannot create directory '%s': Not a directory", dir)
			}
			break
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot create directory '%s': %v", dir, unwrapPathError(err))
		}
		needCreate = append(needCreate, cur)
		parent := filepath.Dir(cur)
		if parent == cur {
			break // reached root
		}
		cur = parent
	}

	// Create directories top-down (reverse order).
	for i := len(needCreate) - 1; i >= 0; i-- {
		d := needCreate[i]
		if err := os.Mkdir(d, 0o777); err != nil {
			if os.IsExist(err) {
				continue // R2.2, R2.3: existing is not an error with -p
			}
			return fmt.Errorf("cannot create directory '%s': %v", d, unwrapPathError(err))
		}
		if verbose {
			// R3.4: GNU mkdir verbose output goes to stdout.
			fmt.Fprintf(os.Stdout, "%s: created directory '%s'\n", progName, d)
		}
	}

	// R3.3: apply specified mode only to the final target directory.
	if modeSpecified && len(needCreate) > 0 {
		if err := os.Chmod(cleaned, mode); err != nil {
			return fmt.Errorf("setting permissions of '%s': %v", dir, unwrapPathError(err))
		}
	}

	return nil
}

// parseMode parses an octal or symbolic permission mode string.
// R3.1: supports both octal (0755) and symbolic (u=rwx,go=rx) formats.
func parseMode(s string) (os.FileMode, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty mode")
	}
	// Try octal if starts with a digit.
	if s[0] >= '0' && s[0] <= '9' {
		n, err := strconv.ParseUint(s, 8, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid octal mode: %s", s)
		}
		return os.FileMode(n), nil
	}
	return parseSymbolicMode(s)
}

// parseSymbolicMode parses a symbolic mode string like "u=rwx,go=rx".
// The base mode is 0 (no permissions), matching GNU mkdir behavior for
// new directory creation.
func parseSymbolicMode(s string) (os.FileMode, error) {
	var mode os.FileMode
	for _, clause := range strings.Split(s, ",") {
		if len(clause) == 0 {
			return 0, fmt.Errorf("invalid symbolic mode")
		}

		// Parse who: [ugoa]*
		who, pos := scanWho(clause)
		if who == 0 {
			who = whoUser | whoGroup | whoOther
		}

		if pos >= len(clause) {
			return 0, fmt.Errorf("invalid symbolic mode: missing operator")
		}

		op := clause[pos]
		if op != '+' && op != '-' && op != '=' {
			return 0, fmt.Errorf("invalid symbolic mode: bad operator %c", op)
		}
		pos++

		// Parse perms: [rwxXst]*
		var permBits os.FileMode
		var specialBits os.FileMode
		for pos < len(clause) {
			switch clause[pos] {
			case 'r':
				permBits |= 4
			case 'w':
				permBits |= 2
			case 'x', 'X':
				permBits |= 1
			case 's':
				if who&whoUser != 0 {
					specialBits |= os.ModeSetuid
				}
				if who&whoGroup != 0 {
					specialBits |= os.ModeSetgid
				}
			case 't':
				specialBits |= os.ModeSticky
			default:
				return 0, fmt.Errorf("invalid symbolic mode: bad permission %c", clause[pos])
			}
			pos++
		}

		// Build mode bits for specified users.
		var bits os.FileMode
		if who&whoUser != 0 {
			bits |= permBits << 6
		}
		if who&whoGroup != 0 {
			bits |= permBits << 3
		}
		if who&whoOther != 0 {
			bits |= permBits
		}

		switch op {
		case '=':
			var mask os.FileMode
			if who&whoUser != 0 {
				mask |= 0o700
			}
			if who&whoGroup != 0 {
				mask |= 0o070
			}
			if who&whoOther != 0 {
				mask |= 0o007
			}
			mode = (mode &^ mask) | bits
		case '+':
			mode |= bits
		case '-':
			mode &^= bits
		}

		// Apply special bits.
		switch op {
		case '+', '=':
			mode |= specialBits
		case '-':
			mode &^= specialBits
		}
	}
	return mode, nil
}

// scanWho parses the who portion [ugoa]* of a symbolic mode clause.
func scanWho(clause string) (who int, pos int) {
	for pos < len(clause) {
		switch clause[pos] {
		case 'u':
			who |= whoUser
		case 'g':
			who |= whoGroup
		case 'o':
			who |= whoOther
		case 'a':
			who |= whoUser | whoGroup | whoOther
		default:
			return
		}
		pos++
	}
	return
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "File exists" instead of "mkdir foo: file exists".
func unwrapPathError(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// printUsage writes the help text to the given writer.
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: mkdir [OPTION]... DIRECTORY...")
	fmt.Fprintln(w, "Create the DIRECTORY(ies), if they do not already exist.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  -m, --mode=MODE   set file mode (as in chmod), not a=rwx - umask")
	fmt.Fprintln(w, "  -p, --parents     no error if existing, make parent directories as needed")
	fmt.Fprintln(w, "  -v, --verbose     print a message for each created directory")
	fmt.Fprintln(w, "  -Z                set SELinux security context of each created directory")
	fmt.Fprintln(w, "                      to the default type")
	fmt.Fprintln(w, "      --context[=CTX]  like -Z, or if CTX is specified then set the SELinux")
	fmt.Fprintln(w, "                         or SMACK security context to CTX")
	fmt.Fprintln(w, "      --help        display this help and exit")
	fmt.Fprintln(w, "      --version     output version information and exit")
}
