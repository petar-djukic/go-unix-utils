// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd056-cp R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4
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
const programName = "cp"

// preserveAttrs tracks which file attributes to preserve on copy.
type preserveAttrs struct {
	mode       bool
	ownership  bool
	timestamps bool
}

// cpOpts holds all flag state for a cp invocation.
type cpOpts struct {
	recursive      bool
	dereference    bool // -L: follow symlinks in source
	noDerefer      bool // -P: preserve symlinks (default with -r)
	preserve       preserveAttrs
	verbose        bool
	interactive    bool   // -i: prompt before overwrite
	force          bool   // -f: remove destination if can't open, then retry
	noClobber      bool   // -n: do not overwrite existing files
	targetDir      string // -t DIRECTORY: copy all sources into DIRECTORY
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var operands []string
	var opts cpOpts

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--recursive":
			opts.recursive = true
		case arg == "--dereference":
			opts.dereference = true
		case arg == "--no-dereference":
			opts.noDerefer = true
		case arg == "--verbose":
			// R3.4: --verbose long option.
			opts.verbose = true
		case arg == "--interactive":
			// R1.2: --interactive long option.
			opts.interactive = true
		case arg == "--force":
			// R1.3: --force long option.
			opts.force = true
		case arg == "--no-clobber":
			// R1.4: --no-clobber long option.
			opts.noClobber = true
		case arg == "--archive":
			// R3.2: --archive is equivalent to -dR --preserve=all.
			opts.recursive = true
			opts.noDerefer = true
			opts.preserve = preserveAttrs{mode: true, ownership: true, timestamps: true}
		case strings.HasPrefix(arg, "--preserve="):
			// R3.3: --preserve=ATTR_LIST with comma-separated attributes.
			parsePreserveList(arg[len("--preserve="):], &opts.preserve)
		case arg == "--preserve":
			// R3.1: bare --preserve is equivalent to --preserve=mode,ownership,timestamps.
			opts.preserve = preserveAttrs{mode: true, ownership: true, timestamps: true}
		case strings.HasPrefix(arg, "--target-directory="):
			// R4.3: --target-directory=DIRECTORY long option.
			opts.targetDir = arg[len("--target-directory="):]
		case arg == "--target-directory":
			// R4.3: --target-directory DIRECTORY (separate argument).
			if i+1 < len(args) {
				i++
				opts.targetDir = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "%s: option '--target-directory' requires an argument\n", programName)
				os.Exit(1)
			}
		case arg == "--":
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			// Unrecognized long option: ignore for forward compatibility.
			operands = append(operands, arg)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Parse bundled short options.
			flags := arg[1:]
			for j := 0; j < len(flags); j++ {
				switch flags[j] {
				case 'r', 'R':
					opts.recursive = true
				case 'L':
					opts.dereference = true
				case 'P':
					opts.noDerefer = true
				case 'p':
					// R3.1: -p is equivalent to --preserve=mode,ownership,timestamps.
					opts.preserve = preserveAttrs{mode: true, ownership: true, timestamps: true}
				case 'a':
					// R3.2: -a is equivalent to -dR --preserve=all.
					opts.recursive = true
					opts.noDerefer = true
					opts.preserve = preserveAttrs{mode: true, ownership: true, timestamps: true}
				case 'v':
					// R3.4: -v verbose mode.
					opts.verbose = true
				case 'd':
					// -d is equivalent to --no-dereference (part of -a expansion).
					opts.noDerefer = true
				case 'i':
					// R1.2: -i interactive mode.
					opts.interactive = true
				case 'f':
					// R1.3: -f force mode.
					opts.force = true
				case 'n':
					// R1.4: -n no-clobber mode.
					opts.noClobber = true
				case 't':
					// R4.3: -t DIRECTORY short option.
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
				default:
					// Ignore unrecognized short flags for forward compatibility.
				}
			}
		default:
			operands = append(operands, arg)
		}
	}

	// R2.4: -P is the default with -r when neither -L nor -P is explicit.
	if opts.recursive && !opts.dereference && !opts.noDerefer {
		opts.noDerefer = true
	}
	// -L overrides -P when both are given (last one wins in GNU, but we
	// resolve: if dereference is set, noDerefer is cleared).
	if opts.dereference {
		opts.noDerefer = false
	}

	// R4.3: -t DIRECTORY makes all operands sources, copied into DIRECTORY.
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
		if len(operands) < 2 {
			fmt.Fprintf(os.Stderr, "%s: missing file operand\n", programName)
			os.Exit(1)
		}
		// R1.1: last argument is the destination; all preceding are sources.
		dest = operands[len(operands)-1]
		sources = operands[:len(operands)-1]
	}

	destInfo, destErr := os.Stat(dest)
	destIsDir := destErr == nil && destInfo.IsDir()

	// R4.3: -t requires the target to be an existing directory.
	if opts.targetDir != "" && !destIsDir {
		fmt.Fprintf(os.Stderr, "%s: target directory '%s': No such file or directory\n", programName, dest)
		os.Exit(1)
	}

	// R1.1: multiple sources require destination to be a directory.
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(os.Stderr, "%s: target '%s' is not a directory\n", programName, dest)
		os.Exit(1)
	}

	// R4.1, R4.2: exit 0 on success, exit 1 on any failure.
	exitCode := 0
	for _, src := range sources {
		target := dest
		if destIsDir {
			target = filepath.Join(dest, filepath.Base(src))
		}
		if err := copyEntry(src, target, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
			exitCode = 1
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// copyEntry copies src to dst. For directories, it recurses when opts.recursive
// is set. For symlinks, it either follows or preserves based on opts.
//
// R2.1: recursive directory copying.
// R2.2: without -r, directories produce an error.
// R2.3: -L follows symlinks.
// R2.4: -P preserves symlinks.
func copyEntry(src, dst string, opts cpOpts) error {
	// Use Lstat to detect symlinks before deciding whether to follow them.
	srcLstat, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}

	// R3.4: verbose output to stdout before the copy, matching GNU cp.
	if opts.verbose {
		fmt.Printf("'%s' -> '%s'\n", src, dst)
	}

	// Handle symlinks.
	if srcLstat.Mode()&os.ModeSymlink != 0 {
		if opts.noDerefer {
			// R2.4: copy as symlink.
			return copySymlink(src, dst)
		}
		// R2.3: follow the symlink — stat to get the target info.
		srcStat, statErr := os.Stat(src)
		if statErr != nil {
			return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(statErr))
		}
		srcLstat = srcStat
	}

	if srcLstat.IsDir() {
		// R2.2: without -r, refuse to copy a directory.
		if !opts.recursive {
			return fmt.Errorf("-r not specified; omitting directory '%s'", src)
		}
		// R2.1: recursive directory copy.
		return copyDir(src, dst, opts)
	}

	// R1.4: -n prevents overwriting existing files. When -n and -i are
	// combined, -n takes precedence.
	if opts.noClobber {
		if _, statErr := os.Lstat(dst); statErr == nil {
			return nil
		}
	}

	// R1.2: -i prompts before overwriting an existing destination file.
	if opts.interactive && !opts.noClobber {
		if _, statErr := os.Lstat(dst); statErr == nil {
			fmt.Fprintf(os.Stderr, "%s: overwrite '%s'? ", programName, dst)
			reader := bufio.NewReader(os.Stdin)
			response, readErr := reader.ReadString('\n')
			if readErr != nil || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(response)), "y") {
				return nil
			}
		}
	}

	// R1.3: -f removes destination if it cannot be opened for writing,
	// then retries the copy.
	if err := copyFile(src, dst, opts.force); err != nil {
		return err
	}
	// R3.1, R3.3: apply attribute preservation after successful file copy.
	return applyPreservation(src, dst, opts.preserve)
}

// copyDir recursively copies the directory at src to dst.
//
// R2.1: preserves directory structure.
func copyDir(src, dst string, opts cpOpts) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}

	// Create the destination directory with the same permissions.
	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("cannot create directory '%s': %s", dst, sysErrMsg(err))
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("cannot open directory '%s' for reading: %s", src, sysErrMsg(err))
	}

	var firstErr error
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if copyErr := copyEntry(srcPath, dstPath, opts); copyErr != nil {
			if firstErr == nil {
				firstErr = copyErr
			} else {
				fmt.Fprintf(os.Stderr, "%s: %v\n", programName, copyErr)
			}
		}
	}

	// R3.1, R3.3: apply attribute preservation to the directory itself after
	// its contents have been copied (timestamps would be reset by child writes).
	if presErr := applyPreservation(src, dst, opts.preserve); presErr != nil {
		if firstErr == nil {
			firstErr = presErr
		}
	}

	return firstErr
}

// copySymlink copies a symbolic link from src to dst, preserving the link target.
//
// R2.4: the symlink is recreated at dst pointing to the same target.
func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("cannot read link '%s': %s", src, sysErrMsg(err))
	}
	if err := os.Symlink(target, dst); err != nil {
		return fmt.Errorf("cannot create symbolic link '%s': %s", dst, sysErrMsg(err))
	}
	return nil
}

// copyFile copies the regular file at src to dst using streaming I/O.
// R1.1: preserves file content byte-for-byte.
// R1.3: when force is true, removes destination and retries if it cannot be opened.
func copyFile(src, dst string, force bool) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}

	// R1.3: detect source equals destination.
	srcAbs, err1 := filepath.Abs(src)
	dstAbs, err2 := filepath.Abs(dst)
	if err1 == nil && err2 == nil && srcAbs == dstAbs {
		return fmt.Errorf("'%s' and '%s' are the same file", src, dst)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open '%s' for reading: %s", src, sysErrMsg(err))
	}
	defer in.Close() // best-effort cleanup, error ignored

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		// R1.3: -f removes destination and retries when the file cannot be opened.
		if force {
			if rmErr := os.Remove(dst); rmErr == nil {
				out, err = os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
			}
		}
		if err != nil {
			return fmt.Errorf("cannot create regular file '%s': %s", dst, sysErrMsg(err))
		}
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

// parsePreserveList parses a comma-separated attribute list and sets the
// corresponding fields on attrs.
//
// R3.3: supported attributes are mode, ownership, timestamps, links, all.
func parsePreserveList(list string, attrs *preserveAttrs) {
	for _, attr := range strings.Split(list, ",") {
		switch strings.TrimSpace(attr) {
		case "mode":
			attrs.mode = true
		case "ownership":
			attrs.ownership = true
		case "timestamps":
			attrs.timestamps = true
		case "links":
			// R3.3: links attribute is accepted but hard link preservation
			// is not implemented (no-op for compatibility).
		case "all":
			attrs.mode = true
			attrs.ownership = true
			attrs.timestamps = true
		}
	}
}

// applyPreservation sets the preserved attributes on dst based on src metadata.
//
// R3.1: -p preserves mode, ownership, and timestamps.
// R3.3: --preserve=ATTR_LIST selectively preserves attributes.
func applyPreservation(src, dst string, attrs preserveAttrs) error {
	if !attrs.mode && !attrs.ownership && !attrs.timestamps {
		return nil
	}

	srcFI, err := sys.Lstat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s", src, sysErrMsg(err))
	}

	isSymlink := srcFI.Mode&os.ModeSymlink != 0

	// R3.1: preserve file mode bits.
	if attrs.mode && !isSymlink {
		if chErr := os.Chmod(dst, srcFI.Mode.Perm()); chErr != nil {
			return fmt.Errorf("preserving permissions for '%s': %s", dst, sysErrMsg(chErr))
		}
	}

	// R3.1: preserve ownership (uid/gid).
	if attrs.ownership {
		if chErr := os.Lchown(dst, int(srcFI.Uid), int(srcFI.Gid)); chErr != nil {
			// Ownership preservation may fail without root; best-effort.
			fmt.Fprintf(os.Stderr, "%s: preserving ownership for '%s': %s\n",
				programName, dst, sysErrMsg(chErr))
		}
	}

	// R3.1: preserve modification and access timestamps.
	if attrs.timestamps && !isSymlink {
		if chErr := os.Chtimes(dst, srcFI.AccessTime, srcFI.ModTime); chErr != nil {
			return fmt.Errorf("preserving times for '%s': %s", dst, sysErrMsg(chErr))
		}
	}

	return nil
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
