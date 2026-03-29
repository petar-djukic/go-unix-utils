// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/env implements GNU env: run a command in a modified environment.
//
// Implements prd039-env R1.1, R1.2, R1.3, R2.1.
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

const programName = "env"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// envConfig holds parsed arguments for env.
type envConfig struct {
	ignoreEnv bool
	setVars   []string
	command   string
	cmdArgs   []string
}

// run parses arguments and either prints the environment or executes a command.
func run(args []string, stdout, stderr *os.File) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err) //nolint:errcheck
		return 125
	}
	env := buildEnv(cfg)
	if cfg.command == "" {
		return printEnv(env, stdout)
	}
	return execCommand(cfg.command, cfg.cmdArgs, env, stderr)
}

// parseArgs processes flags, NAME=VALUE pairs, and the optional command.
func parseArgs(args []string) (envConfig, error) {
	var cfg envConfig
	i, err := parseFlags(args, &cfg)
	if err != nil {
		return envConfig{}, err
	}
	i = collectVars(args, i, &cfg)
	if i < len(args) {
		cfg.command = args[i]
		cfg.cmdArgs = args[i+1:]
	}
	return cfg, nil
}

// parseFlags processes option flags and returns the index of the first non-flag.
// R2.1: -i / --ignore-environment sets ignoreEnv.
func parseFlags(args []string, cfg *envConfig) (int, error) {
	for i := range len(args) {
		switch args[i] {
		case "-i", "--ignore-environment":
			cfg.ignoreEnv = true
		case "--":
			return i + 1, nil
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				return 0, fmt.Errorf("unrecognized option '%s'", args[i])
			}
			return i, nil
		}
	}
	return len(args), nil
}

// collectVars gathers NAME=VALUE pairs starting at index i.
// The first argument without '=' marks the start of COMMAND.
func collectVars(args []string, i int, cfg *envConfig) int {
	for i < len(args) {
		if !strings.Contains(args[i], "=") {
			break
		}
		cfg.setVars = append(cfg.setVars, args[i])
		i++
	}
	return i
}

// buildEnv constructs the environment for command execution or printing.
// R2.1: when ignoreEnv is true, starts with an empty environment.
func buildEnv(cfg envConfig) []string {
	var env []string
	if !cfg.ignoreEnv {
		env = os.Environ()
	}
	return append(env, cfg.setVars...)
}

// printEnv writes each environment variable on its own line.
// R1.1: NAME=VALUE format, one per line, exit 0.
func printEnv(env []string, stdout *os.File) int {
	for _, e := range env {
		fmt.Fprintln(stdout, e) //nolint:errcheck // best-effort
	}
	return 0
}

// execCommand replaces the current process with the named command.
// R1.2: execute COMMAND with the resulting environment.
// R1.3: exit 127 if not found, 126 if not executable.
func execCommand(name string, args, env []string, stderr *os.File) int {
	path, err := exec.LookPath(name)
	if err != nil {
		return handleExecError(name, err, stderr)
	}
	execErr := syscall.Exec(path, append([]string{name}, args...), env)
	return handleExecError(name, execErr, stderr)
}

// handleExecError prints an error and returns the appropriate exit code.
// R1.3: 126 for permission denied, 127 for not found.
func handleExecError(name string, err error, stderr *os.File) int {
	fmt.Fprintf(stderr, "%s: '%s': %v\n", programName, name, err) //nolint:errcheck
	if errors.Is(err, syscall.EACCES) || errors.Is(err, os.ErrPermission) {
		return 126
	}
	return 127
}
