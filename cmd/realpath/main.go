// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd049-realpath R1.1-R1.5, R2.1-R2.3, R3.1-R3.3, R4.1-R4.3:
// cmd/realpath resolves each command-line path argument to its canonical
// absolute pathname, prints one per line, and reports errors for nonexistent
// paths. Supports -e (all must exist), -m (none must exist), -s (no symlink
// resolution), --relative-to, --relative-base, --help, and --version flags.
// Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in diagnostic output.
const progName = "realpath"

func main() {
	// D1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var (
		strip        bool
		modeExisting bool
		modeMissing  bool
		relativeTo   string
		relativeBase string
		paths        []string
	)

	// Parse flags following the pattern established in R1.1-R1.5.
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			paths = append(paths, args[i+1:]...)
			break
		}
		if arg == "-" {
			paths = append(paths, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			// Long flags.
			switch {
			case arg == "--help":
				// R4.2: print usage to stdout and exit 0.
				fmt.Fprintf(os.Stdout, //nolint:errcheck // best-effort output
					"Usage: %s [OPTION]... FILE...\n"+
						"Print the resolved absolute file name;\n"+
						"all but the last component must exist\n\n"+
						"  -e, --canonicalize-existing  all components of the path must exist\n"+
						"  -m, --canonicalize-missing   no path components need exist or be a directory\n"+
						"  -L, --logical                resolve '..' components before symlinks\n"+
						"  -P, --physical               resolve symlinks as encountered (default)\n"+
						"  -q, --quiet                  suppress most error messages\n"+
						"      --relative-to=DIR        print the resolved path relative to DIR\n"+
						"      --relative-base=DIR      print absolute paths unless paths below DIR\n"+
						"  -s, --strip, --no-symlinks   don't expand symlinks\n"+
						"  -z, --zero                   end each output line with NUL, not newline\n"+
						"      --help     display this help and exit\n"+
						"      --version  output version information and exit\n",
					progName,
				)
				os.Exit(0)
			case arg == "--version":
				// R4.1: print version to stdout and exit 0.
				fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // best-effort output
					progName, "go-unix-utils", version.Version,
				)
				os.Exit(0)
			case arg == "--canonicalize-existing":
				// R1.3: all components must exist.
				modeExisting = true
			case arg == "--canonicalize-missing":
				// R1.4: no components need to exist.
				modeMissing = true
			case arg == "--canonicalize":
				// Default mode: all but last component must exist (no-op).
			case arg == "--strip" || arg == "--no-symlinks":
				// R1.5: do not resolve symlinks.
				strip = true
			case arg == "--logical":
				// Logical mode: accepted for compatibility.
			case arg == "--physical":
				// Physical mode: accepted for compatibility.
			case arg == "--quiet":
				// Quiet mode: accepted for compatibility.
			case arg == "--zero":
				// NUL-terminated output: accepted for compatibility.
			case arg == "--relative-to":
				// R2.1: next argument is the directory.
				i++
				if i < len(args) {
					relativeTo = args[i]
				}
			case strings.HasPrefix(arg, "--relative-to="):
				// R2.1: value after '='.
				relativeTo = arg[len("--relative-to="):]
			case arg == "--relative-base":
				// R2.2: next argument is the directory.
				i++
				if i < len(args) {
					relativeBase = args[i]
				}
			case strings.HasPrefix(arg, "--relative-base="):
				// R2.2: value after '='.
				relativeBase = arg[len("--relative-base="):]
			default:
				// R3.2: unknown long flag.
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)                   //nolint:errcheck // best-effort diagnostic
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)               //nolint:errcheck // best-effort diagnostic
				os.Exit(1)
			}
		} else if strings.HasPrefix(arg, "-") {
			// Short flags — process each character.
			for _, c := range arg[1:] {
				switch c {
				case 'e':
					// R1.3: all components must exist.
					modeExisting = true
				case 'm':
					// R1.4: no components need to exist.
					modeMissing = true
				case 's':
					// R1.5: do not resolve symlinks.
					strip = true
				case 'E':
					// Default mode: all but last component must exist (no-op, already default).
				case 'L':
					// Logical mode: resolve .. before symlinks (accepted, not differentiated).
				case 'P':
					// Physical mode: resolve symlinks (default, no-op).
				case 'q':
					// Quiet mode: suppress error messages (accepted for compatibility).
				case 'z':
					// Zero mode: NUL-terminated output (accepted for compatibility).
				default:
					// R3.2: unknown short flag.
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, c)                   //nolint:errcheck // best-effort diagnostic
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)            //nolint:errcheck // best-effort diagnostic
					os.Exit(1)
				}
			}
		} else {
			paths = append(paths, arg)
		}
	}

	// R3.1: no operands → usage error to stderr, exit 1.
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)                   //nolint:errcheck // best-effort diagnostic
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
		os.Exit(1)
	}

	// Resolve the --relative-to and --relative-base directories themselves.
	if relativeTo != "" {
		if r, err := resolvePath(relativeTo, strip, modeExisting, modeMissing); err == nil {
			relativeTo = r
		}
	}
	if relativeBase != "" {
		if r, err := resolvePath(relativeBase, strip, modeExisting, modeMissing); err == nil {
			relativeBase = r
		}
	}

	// R2.2: when only --relative-base is given, it also serves as --relative-to.
	if relativeTo == "" && relativeBase != "" {
		relativeTo = relativeBase
	}

	exitCode := 0

	for _, arg := range paths {
		resolved, err := resolvePath(arg, strip, modeExisting, modeMissing)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, arg, err) //nolint:errcheck // best-effort diagnostic
			exitCode = 1
			continue
		}

		output := makeRelative(resolved, relativeTo, relativeBase)
		fmt.Fprintln(os.Stdout, output) //nolint:errcheck // best-effort output
	}

	os.Exit(exitCode)
}

// resolvePath resolves a path using the appropriate mode.
// When strip is true (R1.5), symlinks are not resolved.
// When modeExisting is true (R1.3: -e), all components must exist.
// When modeMissing is true (R1.4: -m), no components need to exist.
func resolvePath(path string, strip, modeExisting, modeMissing bool) (string, error) {
	if strip {
		return resolveStrip(path)
	}
	if modeMissing {
		return resolveMissing(path)
	}
	if modeExisting {
		return resolveExisting(path)
	}
	return resolve(path)
}

// resolve canonicalizes path to its absolute form with symlinks resolved.
// R1.1: GNU realpath default behavior requires all parent components to exist
// but allows the final component to be missing. It resolves symlinks in the
// existing prefix and appends the remaining base name.
func resolve(path string) (string, error) {
	// Try full resolution first — works when the entire path exists.
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Abs(resolved)
	}

	// R1.1/R1.2: if the full path doesn't exist, resolve the parent directory
	// (which must exist) and append the base name. This matches GNU realpath
	// default behavior where the last component may be missing.
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	resolvedDir, dirErr := filepath.EvalSymlinks(dir)
	if dirErr != nil {
		// Parent doesn't exist — return the original error.
		return "", err
	}

	absDir, dirErr := filepath.Abs(resolvedDir)
	if dirErr != nil {
		return "", dirErr
	}

	return filepath.Join(absDir, base), nil
}

// resolveExisting requires every component of the path to exist (R1.3: -e).
func resolveExisting(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}

// resolveMissing resolves symlinks in the existing prefix and constructs the
// remainder without checking existence (R1.4: -m).
func resolveMissing(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// Try full resolution first.
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}

	// Walk up to find the longest existing prefix, resolve it, then append
	// the missing suffix.
	current := abs
	var suffix []string
	for {
		parent := filepath.Dir(current)
		suffix = append([]string{filepath.Base(current)}, suffix...)
		if parent == current {
			// Reached root — nothing exists, return the cleaned absolute path.
			return filepath.Clean(abs), nil
		}
		current = parent
		resolved, err = filepath.EvalSymlinks(current)
		if err == nil {
			// Existing prefix found — join with the unresolved suffix.
			parts := append([]string{resolved}, suffix...)
			return filepath.Clean(filepath.Join(parts...)), nil
		}
	}
}

// resolveStrip cleans . and .. components and makes the path absolute without
// resolving symlinks (R1.5).
func resolveStrip(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// makeRelative applies --relative-to and --relative-base logic to a resolved path.
// R2.1: when relativeTo is set and relativeBase is empty, all paths are relative.
// R2.2: when relativeBase is set, only paths under relativeBase are relative.
// R2.3: when both are set, relativeBase constrains and relativeTo is the base.
func makeRelative(resolved, relativeTo, relativeBase string) string {
	if relativeTo == "" {
		return resolved
	}

	if relativeBase != "" {
		// R2.2/R2.3: only relativize if the resolved path is below relativeBase.
		if !hasPathPrefix(resolved, relativeBase) {
			return resolved
		}
	}

	rel, err := filepath.Rel(relativeTo, resolved)
	if err != nil {
		return resolved
	}
	return rel
}

// hasPathPrefix reports whether path is equal to or a subdirectory of prefix.
func hasPathPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}
