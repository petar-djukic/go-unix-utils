// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd037-ln R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R3.5, R3.6.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: ln [OPTION]... TARGET LINK_NAME
  or:  ln [OPTION]... TARGET
  or:  ln [OPTION]... TARGET... DIRECTORY
In the 1st form, create a link to TARGET with the name LINK_NAME.
In the 2nd form, create a link to TARGET in the current directory.
In the 3rd form, create links to each TARGET in DIRECTORY.

  -b                         like --backup but does not accept an argument
  -f, --force                remove existing destination files
  -i, --interactive          prompt whether to remove destinations
  -n, --no-dereference       treat LINK_NAME as a normal file if
                              it is a symbolic link to a directory
  -r, --relative             create symbolic links relative to link location
  -s, --symbolic             make symbolic links instead of hard links
  -S, --suffix=SUFFIX        override the usual backup suffix
  -v, --verbose              print name of each linked file
      --backup[=CONTROL]     make a backup of each existing destination file
      --help                 display this help and exit
      --version              output version information and exit
`

const versionText = `ln (go-unix-utils) dev
`

type options struct {
	symbolic      bool
	relative      bool
	force         bool
	interactive   bool
	noDereference bool
	verbose       bool
	backupMethod  string
	suffix        string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, targets, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ln: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'ln --help' for more information.\n")
		os.Exit(1)
	}

	os.Exit(run(opts, targets))
}

func run(opts options, targets []string) int {
	exitCode := 0
	last := targets[len(targets)-1]

	switch {
	case len(targets) == 1:
		linkName := "./" + filepath.Base(targets[0])
		exitCode = doLink(targets[0], linkName, opts, exitCode)
	case isDirDest(last, opts.noDereference):
		for _, target := range targets[:len(targets)-1] {
			linkName := filepath.Join(last, filepath.Base(target))
			exitCode = doLink(target, linkName, opts, exitCode)
		}
	case len(targets) == 2:
		exitCode = doLink(targets[0], targets[1], opts, exitCode)
	default:
		fmt.Fprintf(os.Stderr, "ln: target '%s': %s\n",
			last, targetDirError(last))
		exitCode = 1
	}

	return exitCode
}

func doLink(target, linkName string, opts options, code int) int {
	ok, err := createLink(target, linkName, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ln: %s\n", err)
		return 1
	}
	if !ok {
		return 1
	}
	return code
}

func createLink(target, linkName string, opts options) (bool, error) {
	proceed, backupPath, err := handleExisting(linkName, opts)
	if err != nil {
		return false, err
	}
	if !proceed {
		return false, nil
	}

	if opts.symbolic {
		return true, createSymLink(target, linkName, opts, backupPath)
	}
	return true, createHardLink(target, linkName, opts, backupPath)
}

func createSymLink(target, linkName string, opts options, backupPath string) error {
	t := target
	if opts.relative {
		rel, err := computeRelative(target, linkName)
		if err != nil {
			return err
		}
		t = rel
	}
	if err := os.Symlink(t, linkName); err != nil {
		return fmt.Errorf("failed to create symbolic link '%s': %s",
			linkName, sysErrMsg(err))
	}
	if opts.verbose {
		printVerbose(backupPath, linkName, "->", t)
	}
	return nil
}

func createHardLink(target, linkName string, opts options, backupPath string) error {
	fi, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("failed to access '%s': %s", target, sysErrMsg(err))
	}
	if fi.IsDir() {
		return fmt.Errorf("%s: hard link not allowed for directory", target)
	}
	if err := os.Link(target, linkName); err != nil {
		return fmt.Errorf("failed to create hard link '%s': %s",
			linkName, sysErrMsg(err))
	}
	if opts.verbose {
		printVerbose(backupPath, linkName, "=>", target)
	}
	return nil
}

func printVerbose(backupPath, linkName, arrow, target string) {
	if backupPath != "" {
		fmt.Fprintf(os.Stdout, "'%s' ~ '%s' %s '%s'\n", backupPath, linkName, arrow, target)
	} else {
		fmt.Fprintf(os.Stdout, "'%s' %s '%s'\n", linkName, arrow, target)
	}
}

func handleExisting(linkName string, opts options) (bool, string, error) {
	_, err := os.Lstat(linkName)
	if err != nil {
		return true, "", nil
	}

	if opts.interactive {
		if !confirmReplace(linkName) {
			return false, "", nil
		}
	} else if !opts.force && opts.backupMethod == "" {
		return true, "", nil
	}

	var backupPath string
	if opts.backupMethod != "" {
		backupPath, err = makeBackup(linkName, opts)
		if err != nil {
			return false, "", err
		}
	} else {
		if err := os.Remove(linkName); err != nil {
			return false, "", fmt.Errorf("cannot remove '%s': %s",
				linkName, sysErrMsg(err))
		}
	}

	return true, backupPath, nil
}

func makeBackup(path string, opts options) (string, error) {
	var backupPath string
	switch opts.backupMethod {
	case "numbered":
		backupPath = nextNumberedBackup(path)
	case "existing":
		if hasNumberedBackups(path) {
			backupPath = nextNumberedBackup(path)
		} else {
			backupPath = path + opts.suffix
		}
	default:
		backupPath = path + opts.suffix
	}

	if err := os.Rename(path, backupPath); err != nil {
		return "", fmt.Errorf("cannot backup '%s' to '%s': %s",
			path, backupPath, sysErrMsg(err))
	}
	return backupPath, nil
}

func nextNumberedBackup(path string) string {
	for i := 1; ; i++ {
		bp := fmt.Sprintf("%s.~%d~", path, i)
		if _, err := os.Lstat(bp); err != nil {
			return bp
		}
	}
}

func hasNumberedBackups(path string) bool {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	prefix := base + ".~"
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, "~") {
			middle := name[len(prefix) : len(name)-1]
			if len(middle) > 0 && isAllDigits(middle) {
				return true
			}
		}
	}
	return false
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func confirmReplace(dest string) bool {
	if !sys.IsTerminal(os.Stdin.Fd()) {
		return false
	}
	fmt.Fprintf(os.Stderr, "ln: replace '%s'? ", dest)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	line := scanner.Text()
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

func computeRelative(target, linkName string) (string, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("failed to access '%s': %s", target, sysErrMsg(err))
	}
	linkDir := filepath.Dir(linkName)
	absLinkDir, err := filepath.Abs(linkDir)
	if err != nil {
		return "", fmt.Errorf("failed to access '%s': %s", linkDir, sysErrMsg(err))
	}
	rel, err := filepath.Rel(absLinkDir, absTarget)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %s", err)
	}
	return rel, nil
}

func isDirDest(path string, noDereference bool) bool {
	var fi os.FileInfo
	var err error
	if noDereference {
		fi, err = os.Lstat(path)
	} else {
		fi, err = os.Stat(path)
	}
	if err != nil {
		return false
	}
	return fi.IsDir()
}

func targetDirError(path string) string {
	_, err := os.Stat(path)
	if err != nil {
		return "No such file or directory"
	}
	return "Not a directory"
}

func sysErrMsg(err error) string {
	pe, ok := err.(*os.PathError)
	if !ok {
		le, ok := err.(*os.LinkError)
		if !ok {
			return err.Error()
		}
		return errnoMsg(le.Err)
	}
	return errnoMsg(pe.Err)
}

func errnoMsg(err error) string {
	se, ok := err.(syscall.Errno)
	if !ok {
		return err.Error()
	}
	switch se {
	case syscall.EEXIST:
		return "File exists"
	case syscall.ENOENT:
		return "No such file or directory"
	case syscall.EACCES:
		return "Permission denied"
	case syscall.ENOTDIR:
		return "Not a directory"
	case syscall.EPERM:
		return "Operation not permitted"
	case syscall.EXDEV:
		return "Invalid cross-device link"
	default:
		return se.Error()
	}
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var targets []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			targets = append(targets, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(arg, args[i+1:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			consumed, err := parseShortFlags(arg[1:], args[i+1:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += 1 + consumed
			continue
		}
		targets = append(targets, arg)
		i++
	}

	if len(targets) == 0 {
		return opts, nil, fmt.Errorf("missing file operand")
	}

	if opts.suffix == "" {
		opts.suffix = "~"
	}

	return opts, targets, nil
}

func parseLongFlag(flag string, remaining []string, opts *options) (int, error) {
	switch {
	case flag == "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
		return 0, nil
	case flag == "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
		return 0, nil
	case flag == "--symbolic":
		opts.symbolic = true
		return 1, nil
	case flag == "--relative":
		opts.relative = true
		return 1, nil
	case flag == "--force":
		opts.force = true
		opts.interactive = false
		return 1, nil
	case flag == "--interactive":
		opts.interactive = true
		opts.force = false
		return 1, nil
	case flag == "--no-dereference":
		opts.noDereference = true
		return 1, nil
	case flag == "--verbose":
		opts.verbose = true
		return 1, nil
	case flag == "--backup":
		opts.backupMethod = "existing"
		return 1, nil
	case strings.HasPrefix(flag, "--backup="):
		method := flag[len("--backup="):]
		normalized, err := normalizeBackupMethod(method)
		if err != nil {
			return 0, err
		}
		opts.backupMethod = normalized
		return 1, nil
	case flag == "--suffix":
		if len(remaining) == 0 {
			return 0, fmt.Errorf("option '--suffix' requires an argument")
		}
		opts.suffix = remaining[0]
		return 2, nil
	case strings.HasPrefix(flag, "--suffix="):
		opts.suffix = flag[len("--suffix="):]
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, remaining []string, opts *options) (int, error) {
	consumed := 0
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 's':
			opts.symbolic = true
		case 'r':
			opts.relative = true
		case 'f':
			opts.force = true
			opts.interactive = false
		case 'i':
			opts.interactive = true
			opts.force = false
		case 'n':
			opts.noDereference = true
		case 'v':
			opts.verbose = true
		case 'b':
			opts.backupMethod = "existing"
		case 'S':
			if rest := flags[j+1:]; rest != "" {
				opts.suffix = rest
			} else if len(remaining) > consumed {
				opts.suffix = remaining[consumed]
				consumed++
			} else {
				return 0, fmt.Errorf("option requires an argument -- 'S'")
			}
			return consumed, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return consumed, nil
}

func normalizeBackupMethod(method string) (string, error) {
	switch method {
	case "none", "off":
		return "", nil
	case "numbered", "t":
		return "numbered", nil
	case "existing", "nil":
		return "existing", nil
	case "simple", "never":
		return "simple", nil
	default:
		return "", fmt.Errorf("invalid backup type '%s'", method)
	}
}
