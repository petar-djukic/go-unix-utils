// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd039-env R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3:
// env basic environment display, variable assignment, command execution,
// -i/--ignore-environment, -u/--unset, -0/--null, exit code passthrough,
// and invalid option error handling.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error messages.
const progName = "env"

// helpText is the usage message printed for --help.
const helpText = `Usage: env [OPTION]... [-] [NAME=VALUE]... [COMMAND [ARG]...]
Set each NAME to VALUE in the environment and run COMMAND.

  -i, --ignore-environment  start with an empty environment
  -0, --null           end each output line with NUL, not newline
  -u, --unset=NAME     remove variable from the environment
      --help        display this help and exit
      --version     output version information and exit

A mere - implies -i.  If no COMMAND, print the resulting environment.
`

// versionText is the version message printed for --version.
const versionText = `env (go-unix-utils) 1.0
`

// envOptions holds parsed command-line options.
type envOptions struct {
	ignoreEnv bool
	useNull   bool
	unsetVars []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	opts, remaining := parseOptions(args)
	env := buildEnvironment(opts.ignoreEnv)
	env = removeUnsetVars(env, opts.unsetVars)
	env, cmdStart := applyAssignments(remaining, env)
	if cmdStart >= len(remaining) {
		printEnvironment(env, opts.useNull)
		return
	}
	executeCommand(remaining[cmdStart:], env)
}

// parseOptions processes option flags, returning parsed options and
// remaining arguments. Dispatches to long and short option parsers.
func parseOptions(args []string) (envOptions, []string) {
	opts := envOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return opts, args[i+1:]
		case arg == "-":
			opts.ignoreEnv = true
		case strings.HasPrefix(arg, "--"):
			i = parseLongOption(args, i, &opts)
		case strings.HasPrefix(arg, "-"):
			i = parseShortFlags(args, i, &opts)
		default:
			return opts, args[i:]
		}
	}
	return opts, nil
}

// parseLongOption handles a single --xxx option. R3.3: exits 125 for unknown.
func parseLongOption(args []string, i int, opts *envOptions) int {
	arg := args[i]
	switch {
	case arg == "--help":
		printAndExit(helpText)
	case arg == "--version":
		printAndExit(versionText)
	case arg == "--ignore-environment":
		opts.ignoreEnv = true
	case arg == "--null":
		opts.useNull = true
	case arg == "--unset" && i+1 < len(args):
		i++
		opts.unsetVars = append(opts.unsetVars, args[i])
	case strings.HasPrefix(arg, "--unset="):
		opts.unsetVars = append(opts.unsetVars, arg[len("--unset="):])
	default:
		exitInvalidOption(arg)
	}
	return i
}

// parseShortFlags handles combined short flags like -i0 or -uNAME.
// R3.3: exits 125 for unknown short flag characters.
func parseShortFlags(args []string, i int, opts *envOptions) int {
	chars := args[i][1:]
	for j := 0; j < len(chars); j++ {
		switch chars[j] {
		case 'i':
			opts.ignoreEnv = true
		case '0':
			opts.useNull = true
		case 'u':
			if j+1 < len(chars) {
				opts.unsetVars = append(opts.unsetVars, chars[j+1:])
			} else if i+1 < len(args) {
				i++
				opts.unsetVars = append(opts.unsetVars, args[i])
			}
			return i
		default:
			exitInvalidShortOption(chars[j])
		}
	}
	return i
}

// exitInvalidOption prints an error for an unrecognized long option and
// exits 125. R3.3.
func exitInvalidOption(opt string) {
	fmt.Fprintf(os.Stderr,
		"%s: unrecognized option '%s'\nTry '%s --help' for more information.\n",
		progName, opt, progName)
	os.Exit(125)
}

// exitInvalidShortOption prints an error for an invalid short option
// character and exits 125. R3.3.
func exitInvalidShortOption(c byte) {
	fmt.Fprintf(os.Stderr,
		"%s: invalid option -- '%c'\nTry '%s --help' for more information.\n",
		progName, c, progName)
	os.Exit(125)
}

// buildEnvironment returns the starting environment: empty slice for -i,
// or the current process environment otherwise. R2.1.
func buildEnvironment(ignoreEnv bool) []string {
	if ignoreEnv {
		return []string{}
	}
	return os.Environ()
}

// removeUnsetVars removes specified variables from the environment. R2.2.
func removeUnsetVars(env []string, names []string) []string {
	if len(names) == 0 {
		return env
	}
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !shouldUnset(e, names) {
			result = append(result, e)
		}
	}
	return result
}

// shouldUnset checks if an env entry matches any of the unset names.
func shouldUnset(entry string, names []string) bool {
	for _, name := range names {
		if strings.HasPrefix(entry, name+"=") {
			return true
		}
	}
	return false
}

// applyAssignments processes NAME=VALUE arguments, adding them to env.
// Returns the updated env and the index of the first non-assignment arg. R2.3.
func applyAssignments(args []string, env []string) ([]string, int) {
	i := 0
	for i < len(args) && strings.Contains(args[i], "=") {
		env = setOrAppendEnv(env, args[i])
		i++
	}
	return env, i
}

// setOrAppendEnv sets or appends a NAME=VALUE pair in the env slice.
func setOrAppendEnv(env []string, assignment string) []string {
	idx := strings.IndexByte(assignment, '=')
	prefix := assignment[:idx+1]
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = assignment
			return env
		}
	}
	// Prepend new variables to match GNU env environ ordering.
	return append([]string{assignment}, env...)
}

// printEnvironment writes each env entry to stdout. R1.1, R3.1.
func printEnvironment(env []string, useNull bool) {
	terminator := "\n"
	if useNull {
		terminator = "\x00"
	}
	for _, e := range env {
		fmt.Print(e + terminator)
	}
}

// printAndExit writes text to stdout and exits 0 on success.
func printAndExit(text string) {
	if _, err := fmt.Fprint(os.Stdout, text); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// executeCommand runs the command with the modified environment.
// R1.2 (command execution), R1.3 (exit code 127/126), R3.2 (exit passthrough).
func executeCommand(cmdArgs []string, env []string) {
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		handleExecError(cmdArgs[0], err)
	}
}

// handleExecError processes command execution errors, setting the
// appropriate exit code: 127 for not found, 126 for not executable. R3.2.
func handleExecError(name string, err error) {
	if exitErr, ok := err.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode())
	}
	fmt.Fprintf(os.Stderr, "%s: '%s': %s\n", progName, name, extractOSError(err))
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		os.Exit(127)
	}
	os.Exit(126)
}

// extractOSError extracts the underlying OS error message from an exec error.
func extractOSError(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	if errors.Is(err, exec.ErrNotFound) {
		return "No such file or directory"
	}
	return err.Error()
}
