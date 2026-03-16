// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd036-mktemp R1.1-R1.4:
// cmd/mktemp creates temporary files with unique names and prints the path
// to stdout. Supports custom templates, --version, and --help.
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

	// R1.1, R1.2: use default template in TMPDIR when no template provided.
	// R1.3: when a template is provided, use it as-is (may be relative to cwd).
	useDefaultTemplate := template == ""
	if useDefaultTemplate {
		template = defaultTemplate
	}

	// R1.3: validate template has at least 3 trailing Xs.
	trailingXs := countTrailingXs(template)
	if trailingXs < minTrailingXs {
		fmt.Fprintf(os.Stderr, "%s: too few X's in template '%s'\n", progName, template)
		os.Exit(1)
	}

	// Determine the parent directory and filename template.
	var dir, fileTemplate string
	if strings.Contains(template, "/") {
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

	// Generate the temporary file name and create it.
	trailingXs = countTrailingXs(fileTemplate)
	path, err := createTempFile(dir, fileTemplate, trailingXs)
	if err != nil {
		// R1.4: exit 1 with diagnostic on stderr.
		fmt.Fprintf(os.Stderr, "%s: failed to create file via template '%s': %v\n", progName, template, err)
		os.Exit(1)
	}

	// R1.1: print the absolute path to stdout.
	fmt.Println(path)
}

// parseArgs separates flags from the optional template argument.
func parseArgs(args []string) (*mktempOptions, string) {
	opts := &mktempOptions{}
	var template string
	flagsDone := false

	for _, arg := range args {
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
			switch arg {
			case "--version":
				opts.showVersion = true
			case "--help":
				opts.showHelp = true
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
// R1.4: returns an error if file creation fails.
func createTempFile(dir, template string, trailingXs int) (string, error) {
	prefix := template[:len(template)-trailingXs]

	// Try up to 100 times to avoid collisions.
	for range 100 {
		suffix, err := randomString(trailingXs)
		if err != nil {
			return "", fmt.Errorf("generating random characters: %w", err)
		}

		name := prefix + suffix
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
