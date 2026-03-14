// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd057-mv R1.1-R1.4, R2.1-R2.4
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "mv"

// overwriteMode controls how mv handles existing destination files.
type overwriteMode int

const (
	// modeForce overwrites without prompting (default).
	modeForce overwriteMode = iota
	// modeInteractive prompts before overwriting.
	modeInteractive
	// modeNoClobber silently skips existing destinations.
	modeNoClobber
)

// mvOpts holds all flag state for a mv invocation.
type mvOpts struct {
	overwrite   overwriteMode
	verbose     bool
	targetDir   string // -t DIRECTORY: move all sources into DIRECTORY
	noTargetDir bool   // -T: treat DEST as a normal file, not a directory
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var operands []string
	var opts mvOpts

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--verbose":
			opts.verbose = true
		case arg == "--interactive":
			opts.overwrite = modeInteractive
		case arg == "--force":
			opts.overwrite = modeForce
		case arg == "--no-clobber":
			opts.overwrite = modeNoClobber
		case strings.HasPrefix(arg, "--target-directory="):
			// R3.2: --target-directory=DIRECTORY long option.
			opts.targetDir = arg[len("--target-directory="):]
		case arg == "--target-directory":
			// R3.2: --target-directory DIRECTORY (separate argument).
			if i+1 < len(args) {
				i++
				opts.targetDir = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "%s: option '--target-directory' requires an argument\n", programName)
				os.Exit(1)
			}
		case arg == "--no-target-directory":
			// R3.3: --no-target-directory long option.
			opts.noTargetDir = true
		case arg == "--version":
			fmt.Println("mv (go-unix-utils) 0.1")
			os.Exit(0)
		case arg == "--help":
			printUsage()
			os.Exit(0)
		case arg == "--":
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			// Unrecognized long option.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Parse bundled short options.
			flags := arg[1:]
			for j := 0; j < len(flags); j++ {
				switch flags[j] {
				case 'v':
					opts.verbose = true
				case 'i':
					// R2.1: -i interactive mode. Last flag wins.
					opts.overwrite = modeInteractive
				case 'f':
					// R2.2: -f force mode. Last flag wins.
					opts.overwrite = modeForce
				case 'n':
					// R2.3: -n no-clobber mode. Last flag wins.
					opts.overwrite = modeNoClobber
				case 't':
					// R3.2: -t DIRECTORY short option.
					if j+1 < len(flags) {
						opts.targetDir = flags[j+1:]
						j = len(flags)
					} else if i+1 < len(args) {
						i++
						opts.targetDir = args[i]
					} else {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 't'\n", programName)
						os.Exit(1)
					}
				case 'T':
					// R3.3: -T no-target-directory.
					opts.noTargetDir = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, flags[j])
					os.Exit(1)
				}
			}
		default:
			operands = append(operands, arg)
		}
	}

	// Determine sources and destination.
	var dest string
	var sources []string

	if opts.targetDir != "" {
		if len(operands) < 1 {
			fmt.Fprintf(os.Stderr, "%s: missing file operand\n", programName)
			os.Exit(1)
		}
		dest = opts.targetDir
		sources = operands
	} else {
		if len(operands) < 1 {
			fmt.Fprintf(os.Stderr, "%s: missing file operand\n", programName)
			os.Exit(1)
		}
		if len(operands) < 2 {
			fmt.Fprintf(os.Stderr, "%s: missing destination file operand after '%s'\n", programName, operands[0])
			os.Exit(1)
		}
		// R1.1: last argument is the destination; all preceding are sources.
		dest = operands[len(operands)-1]
		sources = operands[:len(operands)-1]
	}

	destInfo, destErr := os.Stat(dest)
	destIsDir := destErr == nil && destInfo.IsDir()

	// R3.3: -T treats destination as a normal file.
	if opts.noTargetDir {
		destIsDir = false
	}

	// -t requires the target to be an existing directory.
	if opts.targetDir != "" && !destIsDir {
		fmt.Fprintf(os.Stderr, "%s: target directory '%s': No such file or directory\n", programName, dest)
		os.Exit(1)
	}

	// R1.2: multiple sources require destination to be a directory.
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(os.Stderr, "%s: target '%s' is not a directory\n", programName, dest)
		os.Exit(1)
	}

	// R4.1, R4.2, R4.3: exit 0 on success, exit 1 on any failure, continue on partial failure.
	exitCode := 0
	for _, src := range sources {
		target := dest
		// R1.4: when dest is an existing directory, move source into it.
		if destIsDir {
			target = filepath.Join(dest, filepath.Base(src))
		}
		if err := moveEntry(src, target, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
			exitCode = 1
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// moveEntry moves src to dst, handling overwrite control and cross-device fallback.
//
// R1.1: rename on same filesystem, copy-then-remove on cross-device.
// R2.1-R2.3: overwrite control via opts.overwrite.
// R2.2: -f removes read-only destination before moving.
func moveEntry(src, dst string, opts mvOpts) error {
	// Check if source exists.
	_, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}

	// R2.3: -n prevents overwriting existing files.
	if opts.overwrite == modeNoClobber {
		if _, statErr := os.Lstat(dst); statErr == nil {
			return nil
		}
	}

	// R2.1: -i prompts before overwriting an existing destination file.
	if opts.overwrite == modeInteractive {
		if _, statErr := os.Lstat(dst); statErr == nil {
			fmt.Fprintf(os.Stderr, "%s: overwrite '%s'? ", programName, dst)
			reader := bufio.NewReader(os.Stdin)
			response, readErr := reader.ReadString('\n')
			if readErr != nil || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(response)), "y") {
				return nil
			}
		}
	}

	// R2.2: -f removes a read-only destination before moving, without prompting.
	if opts.overwrite == modeForce {
		if dstInfo, statErr := os.Lstat(dst); statErr == nil {
			if dstInfo.Mode().Perm()&0o200 == 0 {
				os.Remove(dst) // best-effort removal; rename below will report any remaining error
			}
		}
	}

	// R3.1: verbose output to stdout, matching GNU mv.
	if opts.verbose {
		fmt.Printf("renamed '%s' -> '%s'\n", src, dst)
	}

	// R1.1: try os.Rename first (same-filesystem move).
	err = os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// D1: detect cross-device error and fall back to copy-then-remove.
	if errors.Is(err, syscall.EXDEV) {
		return crossDeviceMove(src, dst)
	}

	return fmt.Errorf("cannot move '%s' to '%s': %s", src, dst, sysErrMsg(err))
}

// crossDeviceMove copies src to dst and removes src, handling both files and directories.
// Preserves metadata (mode, timestamps, ownership where possible).
//
// R1.1: cross-device fallback via copy-then-remove.
// R1.3: directories are moved without requiring a recursive flag.
// R2.1: preserves all metadata on cross-device move.
// R2.4: cleans up partial destination on copy failure; source is not removed.
func crossDeviceMove(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}

	if srcInfo.IsDir() {
		if err := crossDeviceCopyDir(src, dst); err != nil {
			// R2.4: clean up partial destination on failure.
			os.RemoveAll(dst) // best-effort cleanup
			return err
		}
		return os.RemoveAll(src)
	}

	if srcInfo.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(src)
		if readErr != nil {
			return fmt.Errorf("cannot read link '%s': %s", src, sysErrMsg(readErr))
		}
		if err := os.Symlink(target, dst); err != nil {
			return fmt.Errorf("cannot create symbolic link '%s': %s", dst, sysErrMsg(err))
		}
		return os.Remove(src)
	}

	if err := copyFileContent(src, dst, srcInfo.Mode().Perm()); err != nil {
		// R2.4: clean up partial destination on failure.
		os.Remove(dst) // best-effort cleanup
		return err
	}
	// R2.1: preserve timestamps and ownership after successful copy.
	preserveMetadata(src, dst)
	return os.Remove(src)
}

// crossDeviceCopyDir recursively copies a directory for cross-device moves.
// R2.1: preserves directory metadata (timestamps, ownership) after children are copied.
func crossDeviceCopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}

	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("cannot create directory '%s': %s", dst, sysErrMsg(err))
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("cannot open directory '%s' for reading: %s", src, sysErrMsg(err))
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if err := crossDeviceMove(srcPath, dstPath); err != nil {
			return err
		}
	}
	// R2.1: preserve directory metadata after all children are copied
	// (child writes would reset timestamps if done earlier).
	preserveMetadata(src, dst)
	return nil
}

// preserveMetadata copies timestamps and ownership from src to dst.
// Mode is already preserved by copyFileContent via the perm parameter.
// Ownership preservation requires root; failures are best-effort.
//
// R2.1: preserve all metadata on cross-device move.
func preserveMetadata(src, dst string) {
	srcFI, err := sys.Lstat(src)
	if err != nil {
		return // best-effort: metadata stat failure is not fatal for mv
	}
	isSymlink := srcFI.Mode&os.ModeSymlink != 0

	// Preserve timestamps.
	if !isSymlink {
		os.Chtimes(dst, srcFI.AccessTime, srcFI.ModTime) // best-effort
	}

	// Preserve ownership (requires root).
	os.Lchown(dst, int(srcFI.Uid), int(srcFI.Gid)) // best-effort
}

// copyFileContent copies the content and permissions of src to dst.
func copyFileContent(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open '%s' for reading: %s", src, sysErrMsg(err))
	}
	defer in.Close() // best-effort cleanup, error ignored

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("cannot create regular file '%s': %s", dst, sysErrMsg(err))
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close() // best-effort close before returning error
		return fmt.Errorf("writing '%s': %s", dst, sysErrMsg(err))
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("closing '%s': %s", dst, sysErrMsg(err))
	}
	return nil
}

// printUsage prints a brief usage message to stdout.
func printUsage() {
	fmt.Println("Usage: mv [OPTION]... SOURCE DEST")
	fmt.Println("  or:  mv [OPTION]... SOURCE... DIRECTORY")
	fmt.Println("  or:  mv [OPTION]... -t DIRECTORY SOURCE...")
	fmt.Println("Rename SOURCE to DEST, or move SOURCE(s) to DIRECTORY.")
	fmt.Println()
	fmt.Println("  -f, --force           do not prompt before overwriting")
	fmt.Println("  -i, --interactive     prompt before overwrite")
	fmt.Println("  -n, --no-clobber      do not overwrite an existing file")
	fmt.Println("  -v, --verbose         explain what is being done")
	fmt.Println("  -t, --target-directory=DIRECTORY  move all SOURCE arguments into DIRECTORY")
	fmt.Println("  -T, --no-target-directory  treat DEST as a normal file")
	fmt.Println("      --help            display this help and exit")
	fmt.Println("      --version         output version information and exit")
}

// sysErrMsg extracts the underlying syscall error message string from a
// (possibly wrapped) error, producing GNU-compatible messages like
// "No such file or directory" rather than Go's "stat /path: no such file...".
func sysErrMsg(err error) string {
	var msg string
	var errno syscall.Errno
	if errors.As(err, &errno) {
		msg = errno.Error()
	} else {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			msg = pathErr.Err.Error()
		} else {
			msg = err.Error()
		}
	}
	return capitalizeFirst(msg)
}

// capitalizeFirst returns s with its first rune uppercased, matching GNU
// coreutils error message capitalization (e.g., "No such file or directory").
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}
