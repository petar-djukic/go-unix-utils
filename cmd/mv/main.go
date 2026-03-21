// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd057-mv R1.1–R1.4: basic file move, rename, and argument handling.
// Implements prd057-mv R2.1–R2.4: overwrite control (interactive, force, no-clobber).
// Implements prd057-mv R3.1: verbose mode.
// Implements prd057-mv R3.2: target directory (-t/--target-directory).
// Implements prd057-mv R3.3: no-target-directory (-T/--no-target-directory).
// Implements prd057-mv R4.1: exit 0 on success.
// Implements prd057-mv R4.2: exit 1 on failure.
// Implements prd057-mv R4.3: continue on multi-file failure, exit 1.
// TODO: Task requested --backup (R3.2) and --suffix (R3.3), but prd057-mv non_goals
// explicitly exclude --backup and --suffix. Skipped per E6 non-goals enforcement.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "mv"

// overwriteMode controls how existing destination files are handled. R2.1–R2.3.
type overwriteMode int

const (
	modeDefault     overwriteMode = iota
	modeInteractive               // R2.1: prompt before overwriting
	modeForce                     // R2.2: never prompt
	modeNoClobber                 // R2.3: never overwrite
)

// mvConfig holds parsed flag state.
type mvConfig struct {
	mode        overwriteMode
	verbose     bool   // R3.1
	targetDir   string // R3.2
	noTargetDir bool   // R3.3
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and executes the move operation, returning the exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if code := validateFlags(cfg, stderr); code >= 0 {
		return code
	}
	reader := bufio.NewReader(stdin)
	if cfg.targetDir != "" {
		return runWithTargetDir(files, cfg, reader, stderr)
	}
	if err := validateOperands(files, stderr); err != nil {
		return 1
	}
	return doMove(files, cfg, reader, stderr)
}

// validateFlags checks for incompatible flag combinations. R3.2, R3.3.
func validateFlags(cfg mvConfig, stderr io.Writer) int {
	if cfg.targetDir != "" && cfg.noTargetDir {
		fmt.Fprintf(stderr,
			"%s: cannot combine --target-directory (-t) and --no-target-directory (-T)\n",
			progName)
		printTryHelp(stderr)
		return 1
	}
	return -1
}

// runWithTargetDir handles -t mode: all positional args are sources. R3.2.
func runWithTargetDir(files []string, cfg mvConfig, stdin *bufio.Reader, stderr io.Writer) int {
	if len(files) == 0 {
		fmt.Fprintf(stderr, "%s: missing file operand\n", progName)
		printTryHelp(stderr)
		return 1
	}
	return moveAll(files, cfg.targetDir, true, cfg, stdin, stderr)
}

// validateOperands checks that enough file arguments were provided. R1.4.
func validateOperands(files []string, stderr io.Writer) error {
	if len(files) == 0 {
		fmt.Fprintf(stderr, "%s: missing file operand\n", progName)
		printTryHelp(stderr)
		return fmt.Errorf("missing operand")
	}
	if len(files) == 1 {
		fmt.Fprintf(stderr, "%s: missing destination file operand after '%s'\n",
			progName, files[0])
		printTryHelp(stderr)
		return fmt.Errorf("missing destination")
	}
	return nil
}

// doMove moves each source to the destination. R1.1, R1.2, R1.4, R3.3.
func doMove(files []string, cfg mvConfig, stdin *bufio.Reader, stderr io.Writer) int {
	dest := files[len(files)-1]
	sources := files[:len(files)-1]
	if cfg.noTargetDir {
		return doMoveNoTargetDir(sources, dest, cfg, stdin, stderr)
	}
	destInfo, err := os.Stat(dest)
	destIsDir := err == nil && destInfo.IsDir()
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(stderr, "%s: target '%s': Not a directory\n", progName, dest)
		return 1
	}
	return moveAll(sources, dest, destIsDir, cfg, stdin, stderr)
}

// doMoveNoTargetDir handles -T: treat dest as a normal file. R3.3.
func doMoveNoTargetDir(sources []string, dest string, cfg mvConfig, stdin *bufio.Reader, stderr io.Writer) int {
	if len(sources) > 1 {
		fmt.Fprintf(stderr, "%s: extra operand '%s'\n", progName, sources[1])
		printTryHelp(stderr)
		return 1
	}
	return moveAll(sources, dest, false, cfg, stdin, stderr)
}

// moveAll iterates over sources and moves each one. R4.3: continues on error.
func moveAll(sources []string, dest string, destIsDir bool, cfg mvConfig, stdin *bufio.Reader, stderr io.Writer) int {
	exitCode := 0
	for _, src := range sources {
		target := dest
		if destIsDir {
			target = filepath.Join(dest, filepath.Base(src))
		}
		if err := moveSingle(src, target, cfg, stdin, stderr); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// moveSingle moves one source to the target path. R1.1, R1.3, R2.1–R2.4, R3.1.
func moveSingle(src, dest string, cfg mvConfig, stdin *bufio.Reader, stderr io.Writer) error {
	if _, err := os.Lstat(src); err != nil {
		fmt.Fprintf(stderr, "%s: cannot stat '%s': %s\n",
			progName, src, unwrapPathError(err))
		return err
	}
	proceed, err := checkOverwrite(dest, cfg, stdin, stderr)
	if !proceed {
		return err
	}
	if err := os.Rename(src, dest); err != nil {
		fmt.Fprintf(stderr, "%s: cannot move '%s' to '%s': %s\n",
			progName, src, dest, unwrapPathError(err))
		return err
	}
	if cfg.verbose {
		// R3.1: print verbose to stderr matching GNU gmv format
		fmt.Fprintf(stderr, "renamed '%s' -> '%s'\n", src, dest)
	}
	return nil
}

// checkOverwrite checks whether dest can be overwritten based on the
// overwrite mode. Returns (true, nil) to proceed, (false, nil) for
// silent skip (no-clobber), or (false, err) for interactive decline.
// R2.1–R2.3.
func checkOverwrite(dest string, cfg mvConfig, stdin *bufio.Reader, stderr io.Writer) (bool, error) {
	if _, err := os.Lstat(dest); err != nil {
		return true, nil // dest doesn't exist, always proceed
	}
	switch cfg.mode {
	case modeNoClobber:
		return false, nil // R2.3: skip silently
	case modeForce:
		return true, nil // R2.2
	case modeInteractive:
		if promptOverwrite(dest, stdin, stderr) {
			return true, nil
		}
		return false, fmt.Errorf("not overwritten") // R2.1
	default:
		return true, nil
	}
}

// promptOverwrite asks the user whether to overwrite dest. R2.1.
func promptOverwrite(dest string, stdin *bufio.Reader, stderr io.Writer) bool {
	fmt.Fprintf(stderr, "%s: overwrite '%s'? ", progName, dest)
	line, err := stdin.ReadString('\n')
	if err != nil && len(line) == 0 {
		return false
	}
	response := strings.ToLower(strings.TrimSpace(line))
	return response == "y" || response == "yes"
}

// parseArgs separates flags from file arguments and builds config.
// Returns config, file list, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (mvConfig, []string, int) {
	var cfg mvConfig
	var files []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if len(arg) > 2 && arg[1] == '-' {
			consumed, code := applyLongFlag(arg, args, i, &cfg, stdout, stderr)
			if code >= 0 {
				return cfg, nil, code
			}
			i += consumed
			continue
		}
		consumed, code := applyShortFlags(arg, args, i, &cfg, stderr)
		if code >= 0 {
			return cfg, nil, code
		}
		i += consumed
	}
	return cfg, files, -1
}

// applyShortFlags processes combined short flags. R2.2: last overwrite flag wins.
// Returns (consumed, code): consumed is extra args used, code >= 0 is terminal.
func applyShortFlags(arg string, args []string, idx int, cfg *mvConfig, stderr io.Writer) (int, int) {
	consumed := 0
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'i':
			cfg.mode = modeInteractive
		case 'f':
			cfg.mode = modeForce
		case 'n':
			cfg.mode = modeNoClobber
		case 'v':
			cfg.verbose = true
		case 't':
			val, extra, ok := consumeFlagValue(arg, args, j, idx, consumed)
			if !ok {
				fmt.Fprintf(stderr, "%s: option requires an argument -- 't'\n", progName)
				printTryHelp(stderr)
				return 0, 1
			}
			cfg.targetDir = val
			return consumed + extra, -1
		case 'T':
			cfg.noTargetDir = true
		default:
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n",
				progName, arg[j])
			printTryHelp(stderr)
			return 0, 1
		}
	}
	return consumed, -1
}

// consumeFlagValue extracts the value for a short flag that requires an argument.
// Checks the rest of the current arg first, then the next arg.
func consumeFlagValue(arg string, args []string, charIdx, argIdx, consumed int) (string, int, bool) {
	if charIdx+1 < len(arg) {
		return arg[charIdx+1:], 0, true
	}
	nextIdx := argIdx + 1 + consumed
	if nextIdx < len(args) {
		return args[nextIdx], 1, true
	}
	return "", 0, false
}

// applyLongFlag handles --long-name flags.
// Returns (consumed, code): consumed is extra args used, code >= 0 is terminal.
func applyLongFlag(arg string, args []string, idx int, cfg *mvConfig, stdout, stderr io.Writer) (int, int) {
	switch {
	case arg == "--help":
		printHelp(stdout)
		return 0, 0
	case arg == "--version":
		printVersion(stdout)
		return 0, 0
	case arg == "--interactive":
		cfg.mode = modeInteractive
		return 0, -1
	case arg == "--force":
		cfg.mode = modeForce
		return 0, -1
	case arg == "--no-clobber":
		cfg.mode = modeNoClobber
		return 0, -1
	case arg == "--verbose":
		cfg.verbose = true
		return 0, -1
	case arg == "--no-target-directory":
		cfg.noTargetDir = true
		return 0, -1
	case strings.HasPrefix(arg, "--target-directory="):
		cfg.targetDir = arg[len("--target-directory="):]
		return 0, -1
	case arg == "--target-directory":
		return applyTargetDirLong(args, idx, cfg, stderr)
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 0, 1
	}
}

// applyTargetDirLong handles --target-directory without = (value in next arg).
func applyTargetDirLong(args []string, idx int, cfg *mvConfig, stderr io.Writer) (int, int) {
	if idx+1 < len(args) {
		cfg.targetDir = args[idx+1]
		return 1, -1
	}
	fmt.Fprintf(stderr,
		"%s: option '--target-directory' requires an argument\n", progName)
	printTryHelp(stderr)
	return 0, 1
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... SOURCE DEST\n", progName)
	fmt.Fprintf(w, "  or:  %s [OPTION]... SOURCE... DIRECTORY\n", progName)
	fmt.Fprintf(w, "  or:  %s [OPTION]... -t DIRECTORY SOURCE...\n", progName)
	fmt.Fprintln(w, "Rename SOURCE to DEST, or move SOURCE(s) to DIRECTORY.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -f, --force                    do not prompt before overwriting")
	fmt.Fprintln(w, "  -i, --interactive               prompt before overwrite")
	fmt.Fprintln(w, "  -n, --no-clobber                do not overwrite an existing file")
	fmt.Fprintln(w, "  -t, --target-directory=DIRECTORY  move all SOURCE arguments into DIRECTORY")
	fmt.Fprintln(w, "  -T, --no-target-directory        treat DEST as a normal file")
	fmt.Fprintln(w, "  -v, --verbose                   explain what is being done")
	fmt.Fprintln(w, "      --help                      display this help and exit")
	fmt.Fprintln(w, "      --version                   output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages (e.g., "No such file or directory").
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
