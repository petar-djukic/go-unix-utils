// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd036-mktemp R1.1-R1.5, R2.1-R2.3, R3.1-R3.3:
// cmd/mktemp creates temporary files or directories with unique names and
// prints the path to stdout. Supports custom templates, -d for directory
// creation, --tmpdir/-p for parent directory control, --suffix for appending
// a suffix after the random characters, --version, and --help.
// Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU mktemp format.
const progName = "mktemp"

// defaultTemplate is the template used when no TEMPLATE argument is provided.
// R1.2: default template is tmp.XXXXXXXXXX (10 Xs).
const defaultTemplate = "tmp.XXXXXXXXXX"

// minTrailingXs is the minimum number of trailing X characters required in a template.
// R1.3: template must contain at least 3 consecutive X characters at the end.
const minTrailingXs = 3

// randChars is the character set used for replacing X characters in templates.
const randChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// mktempOptions holds the parsed flags for a mktemp invocation.
type mktempOptions struct {
	showVersion bool
	showHelp    bool
	directory   bool   // R2.1: -d/--directory creates a directory instead of a file.
	tmpdir      string // R3.1: --tmpdir=DIR or -p DIR overrides parent directory.
	tmpdirSet   bool   // True when --tmpdir or -p was explicitly provided.
	suffix      string // R3.3: --suffix=SUFF appended after random characters.
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, template := parseArgs(os.Args[1:])

	if opts.showVersion {
		fmt.Println("mktemp (go-unix-utils) 0.1")
		os.Exit(0)
	}

	if opts.showHelp {
		printUsage(os.Stdout)
		os.Exit(0)
	}

	// R3.3: suffix must not contain a directory separator.
	if strings.Contains(opts.suffix, "/") {
		fmt.Fprintf(os.Stderr, "%s: invalid suffix '%s', contains directory separator\n", progName, opts.suffix)
		os.Exit(1)
	}

	// R1.1, R1.2: use default template in TMPDIR when no template provided.
	// R1.3: when a template is provided, use it as-is (may be relative to cwd).
	useDefaultTemplate := template == ""
	if useDefaultTemplate {
		template = defaultTemplate
	}

	// R1.5: validate template has at least 3 trailing Xs (before suffix is appended).
	trailingXs := countTrailingXs(template)
	if trailingXs < minTrailingXs {
		fmt.Fprintf(os.Stderr, "%s: too few X's in template '%s'\n", progName, template)
		os.Exit(1)
	}

	// Determine the parent directory and filename template.
	var dir, fileTemplate string
	if opts.tmpdirSet {
		// R3.1: --tmpdir=DIR or -p DIR overrides parent directory.
		// R3.2: --tmpdir without value uses TMPDIR or /tmp.
		dir = opts.tmpdir
		if dir == "" {
			dir = os.Getenv("TMPDIR")
			if dir == "" {
				dir = "/tmp"
			}
		}
		fileTemplate = template
	} else if strings.Contains(template, "/") {
		// Template contains a directory component.
		dir = template[:strings.LastIndex(template, "/")]
		fileTemplate = template[strings.LastIndex(template, "/")+1:]
	} else if useDefaultTemplate {
		// R1.1: default template uses TMPDIR or /tmp.
		dir = os.Getenv("TMPDIR")
		if dir == "" {
			dir = "/tmp"
		}
		fileTemplate = template
	} else {
		// Custom template without directory: create in current directory.
		dir = "."
		fileTemplate = template
	}

	// Generate the temporary name and create the file or directory.
	trailingXs = countTrailingXs(fileTemplate)
	if opts.directory {
		// R2.1-R2.3: create a directory with mode 0700.
		path, err := createTempDir(dir, fileTemplate, trailingXs, opts.suffix)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: failed to create directory via template '%s': %v\n", progName, template, err)
			os.Exit(1)
		}
		fmt.Println(path)
	} else {
		// R1.1-R1.5: create a file with mode 0600.
		path, err := createTempFile(dir, fileTemplate, trailingXs, opts.suffix)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: failed to create file via template '%s': %v\n", progName, template, err)
			os.Exit(1)
		}
		fmt.Println(path)
	}
}

// parseArgs separates flags from the optional template argument.
func parseArgs(args []string) (*mktempOptions, string) {
	opts := &mktempOptions{}
	var template string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone {
			template = arg
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--version":
				opts.showVersion = true
			case arg == "--help":
				opts.showHelp = true
			case arg == "--directory":
				opts.directory = true
			case arg == "--tmpdir":
				// R3.2: --tmpdir without =DIR uses TMPDIR or /tmp.
				opts.tmpdirSet = true
				opts.tmpdir = ""
			case strings.HasPrefix(arg, "--tmpdir="):
				// R3.1: --tmpdir=DIR uses DIR as parent.
				opts.tmpdirSet = true
				opts.tmpdir = arg[len("--tmpdir="):]
			case strings.HasPrefix(arg, "--suffix="):
				// R3.3: --suffix=SUFF appended after random chars.
				opts.suffix = arg[len("--suffix="):]
			}
			continue
		}
		// Short options.
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			for j := 1; j < len(arg); j++ {
				switch arg[j] {
				case 'd':
					opts.directory = true
				case 'p':
					// -p DIR: next argument or rest of current arg is the directory.
					rest := arg[j+1:]
					if rest != "" {
						opts.tmpdirSet = true
						opts.tmpdir = rest
					} else if i+1 < len(args) {
						i++
						opts.tmpdirSet = true
						opts.tmpdir = args[i]
					} else {
						opts.tmpdirSet = true
						opts.tmpdir = ""
					}
					j = len(arg) // stop processing this arg
				}
			}
			continue
		}
		// Not a flag — treat as template.
		template = arg
	}
	return opts, template
}

// countTrailingXs returns the number of consecutive 'X' characters at the end
// of the template string.
func countTrailingXs(template string) int {
	count := 0
	for i := len(template) - 1; i >= 0; i-- {
		if template[i] != 'X' {
			break
		}
		count++
	}
	return count
}

// createTempFile generates a unique filename from the template and creates the
// file with mode 0600 in the specified directory.
// R1.4: file permission mode 0600. R1.5: returns error on failure.
func createTempFile(dir, template string, trailingXs int, suffix string) (string, error) {
	prefix := template[:len(template)-trailingXs]

	// Try up to 100 times to avoid collisions.
	for range 100 {
		randStr, err := randomString(trailingXs)
		if err != nil {
			return "", fmt.Errorf("generating random characters: %w", err)
		}

		// R3.3: append suffix after the random characters.
		name := prefix + randStr + suffix
		path := dir + "/" + name

		// R1.4: create file with mode 0600 (owner read-write only).
		// O_EXCL ensures atomic creation (no race condition).
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			if os.IsExist(err) {
				continue // collision, retry
			}
			return "", err
		}
		// best-effort close; file is created, error ignored
		f.Close()
		return path, nil
	}
	return "", fmt.Errorf("exhausted attempts to create unique name")
}

// createTempDir generates a unique directory name from the template and creates
// the directory with mode 0700 in the specified parent directory.
// R2.1: creates directory. R2.2: permission mode 0700. R2.3: returns path.
func createTempDir(dir, template string, trailingXs int, suffix string) (string, error) {
	prefix := template[:len(template)-trailingXs]

	// Try up to 100 times to avoid collisions.
	for range 100 {
		randStr, err := randomString(trailingXs)
		if err != nil {
			return "", fmt.Errorf("generating random characters: %w", err)
		}

		// R3.3: append suffix after the random characters.
		name := prefix + randStr + suffix
		path := dir + "/" + name

		// R2.2: create directory with mode 0700 (owner read-write-execute only).
		err = os.Mkdir(path, 0o700)
		if err != nil {
			if os.IsExist(err) {
				continue // collision, retry
			}
			return "", err
		}
		return path, nil
	}
	return "", fmt.Errorf("exhausted attempts to create unique name")
}

// randomString generates a random string of the given length using randChars.
func randomString(length int) (string, error) {
	max := big.NewInt(int64(len(randChars)))
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = randChars[n.Int64()]
	}
	return string(b), nil
}

// printUsage writes the help text to the given writer.
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: mktemp [OPTION]... [TEMPLATE]")
	fmt.Fprintln(w, "Create a temporary file or directory, safely, and print its name.")
	fmt.Fprintln(w, "TEMPLATE must contain at least 3 consecutive 'X's in last component.")
	fmt.Fprintln(w, "If TEMPLATE is not specified, use tmp.XXXXXXXXXX, and --tmpdir is implied.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  -d, --directory     create a directory, not a file")
	fmt.Fprintln(w, "  -u, --dry-run       do not create anything; merely print a name (unsafe)")
	fmt.Fprintln(w, "  -q, --quiet         suppress diagnostics about file/dir-creation failure")
	fmt.Fprintln(w, "  --suffix=SUFF       append SUFF to TEMPLATE; SUFF must not contain a slash.")
	fmt.Fprintln(w, "                        This option is implied if TEMPLATE does not end in X")
	fmt.Fprintln(w, "  -p DIR, --tmpdir[=DIR]  interpret TEMPLATE relative to DIR; if DIR is not")
	fmt.Fprintln(w, "                        specified, use $TMPDIR if set, else /tmp.  With")
	fmt.Fprintln(w, "                        this option, TEMPLATE must not be an absolute name;")
	fmt.Fprintln(w, "                        unlike with -t, TEMPLATE may contain slashes, but")
	fmt.Fprintln(w, "                        mktemp creates only the final component")
	fmt.Fprintln(w, "  -t                  interpret TEMPLATE as a single file name component,")
	fmt.Fprintln(w, "                        relative to a directory: $TMPDIR, if set; else the")
	fmt.Fprintln(w, "                        directory specified via -p; else /tmp [deprecated]")
	fmt.Fprintln(w, "      --help          display this help and exit")
	fmt.Fprintln(w, "      --version       output version information and exit")
}
