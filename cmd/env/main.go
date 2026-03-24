// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd039-env: Run a Command in a Modified Environment.
// Covers R1.1-R1.3 (default behavior), R2.1-R2.3 (environment modification).
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "env"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// config holds parsed command-line options and arguments.
type config struct {
	ignoreEnv   bool
	unsetVars   []string
	setVars     []string // NAME=VALUE pairs
	command     []string
	showHelp    bool
	showVersion bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and executes the env operation. Returns exit code.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		printTryHelp()
		return 125
	}
	if cfg.showHelp {
		return printHelp()
	}
	if cfg.showVersion {
		return printVersion()
	}
	env := buildEnvironment(cfg)
	if len(cfg.command) == 0 {
		return printEnvironment(env)
	}
	return execCommand(cfg.command, env)
}

// parseArgs splits arguments into options, NAME=VALUE pairs, and COMMAND.
// R2.3: first non-option argument without '=' starts COMMAND.
func parseArgs(args []string) (config, error) {
	var cfg config
	i, err := parseOptions(&cfg, args)
	if err != nil {
		return cfg, err
	}
	if cfg.showHelp || cfg.showVersion {
		return cfg, nil
	}
	for ; i < len(args); i++ {
		if strings.Contains(args[i], "=") {
			cfg.setVars = append(cfg.setVars, args[i])
		} else {
			cfg.command = args[i:]
			break
		}
	}
	return cfg, nil
}

// parseOptions processes command-line flags and returns the index of the
// first non-option argument. Handles -i, -u, --, --help, --version, and
// bare '-' (alias for -i).
func parseOptions(cfg *config, args []string) (int, error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return i + 1, nil
		case arg == "-":
			cfg.ignoreEnv = true
		case arg == "--ignore-environment":
			cfg.ignoreEnv = true
		case arg == "--help":
			cfg.showHelp = true
			return i + 1, nil
		case arg == "--version":
			cfg.showVersion = true
			return i + 1, nil
		case strings.HasPrefix(arg, "--unset="):
			cfg.unsetVars = append(cfg.unsetVars, arg[len("--unset="):])
		case arg == "--unset":
			next, err := requireArg(args, i, "'--unset'")
			if err != nil {
				return 0, err
			}
			cfg.unsetVars = append(cfg.unsetVars, next)
			i++
		case strings.HasPrefix(arg, "--"):
			return 0, fmt.Errorf("unrecognized option '%s'", arg)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			advance, err := parseShortFlags(cfg, args, i)
			if err != nil {
				return 0, err
			}
			i += advance - 1
		default:
			return i, nil
		}
	}
	return len(args), nil
}

// requireArg returns args[i+1] or an error if there is no next argument.
func requireArg(args []string, i int, label string) (string, error) {
	if i+1 >= len(args) {
		return "", fmt.Errorf("option %s requires an argument", label)
	}
	return args[i+1], nil
}

// parseShortFlags processes combined short flags at args[i].
// Returns the number of args consumed (1 or 2 if -u consumed the next arg).
func parseShortFlags(cfg *config, args []string, i int) (int, error) {
	arg := args[i]
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'i':
			cfg.ignoreEnv = true
		case 'u':
			name, consumed, err := consumeFlagArg(args, i, j)
			if err != nil {
				return 0, err
			}
			cfg.unsetVars = append(cfg.unsetVars, name)
			return 1 + consumed, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", arg[j])
		}
	}
	return 1, nil
}

// consumeFlagArg extracts the argument for a short flag that requires a value.
// If there are remaining characters in args[i] after position j, they are the
// value. Otherwise the next argument is consumed.
func consumeFlagArg(args []string, i, j int) (string, int, error) {
	arg := args[i]
	if j+1 < len(arg) {
		return arg[j+1:], 0, nil
	}
	if i+1 >= len(args) {
		return "", 0, fmt.Errorf("option requires an argument -- '%c'", arg[j])
	}
	return args[i+1], 1, nil
}

// buildEnvironment constructs the environment slice from config.
// R2.1: -i starts with an empty environment.
// R2.2: -u removes named variables.
// R2.3: NAME=VALUE pairs set or override variables.
func buildEnvironment(cfg config) []string {
	var env []string
	if !cfg.ignoreEnv {
		env = os.Environ()
	}
	for _, name := range cfg.unsetVars {
		env = removeEnvVar(env, name)
	}
	for _, pair := range cfg.setVars {
		env = setEnvPair(env, pair)
	}
	return env
}

// removeEnvVar removes all entries for the named variable from env.
func removeEnvVar(env []string, name string) []string {
	prefix := name + "="
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}

// setEnvPair sets or replaces a NAME=VALUE entry in env.
func setEnvPair(env []string, pair string) []string {
	idx := strings.IndexByte(pair, '=')
	if idx < 0 {
		return append(env, pair)
	}
	prefix := pair[:idx+1]
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = pair
			return env
		}
	}
	return append(env, pair)
}

// printEnvironment writes each environment entry to stdout.
// R1.1: prints NAME=VALUE per line, exits 0.
func printEnvironment(env []string) int {
	for _, e := range env {
		if _, err := fmt.Fprintln(os.Stdout, e); err != nil {
			return 1
		}
	}
	return 0
}

// execCommand replaces the current process with COMMAND.
// R1.2: executes COMMAND with the resulting environment.
// R1.3: exits 127 if not found, 126 if cannot execute.
func execCommand(command []string, env []string) int {
	path, err := exec.LookPath(command[0])
	if err != nil {
		return handleLookupError(command[0], err)
	}
	execErr := syscall.Exec(path, command, env)
	fmt.Fprintf(os.Stderr, "%s: '%s': %s\n",
		progName, command[0], execErr)
	return 126
}

// handleLookupError distinguishes "not found" (127) from "cannot execute" (126).
func handleLookupError(name string, err error) int {
	if errors.Is(err, os.ErrPermission) {
		fmt.Fprintf(os.Stderr, "%s: '%s': Permission denied\n",
			progName, name)
		return 126
	}
	fmt.Fprintf(os.Stderr, "%s: '%s': No such file or directory\n",
		progName, name)
	return 127
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: env [OPTION]... [-] [NAME=VALUE]... [COMMAND [ARG]...]
Set each NAME to VALUE in the environment and run COMMAND.

  -i, --ignore-environment  start with an empty environment
  -u, --unset=NAME          remove variable from the environment
      --help                display this help and exit
      --version             output version information and exit

A mere - implies -i.  If no COMMAND, print the resulting environment.
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "%s (go-unix-utils) %s\n", progName, version)
	if err != nil {
		return 1
	}
	return 0
}
