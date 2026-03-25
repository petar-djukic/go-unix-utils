// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd089-chmod R1.1, R1.2, R1.3, R1.4, R3.1, R3.2, R4.1, R4.2, R4.3.
// chmod changes file mode bits using octal or symbolic mode specifications.

package main

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R4.3: SIGPIPE handling per shared_protocols.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and applies mode changes. Returns exit code.
func run(args []string) int {
	ref, args := extractReference(args)
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "chmod: missing operand")
		return 1
	}
	resolver, files, err := buildResolver(ref, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return applyToFiles(resolver, files)
}

// modeResolver computes the target mode for a file.
type modeResolver interface {
	resolve(current fs.FileMode) fs.FileMode
}

// extractReference pulls --reference=RFILE from args, returning the
// reference path and remaining args.
func extractReference(args []string) (string, []string) {
	var ref string
	var rest []string
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, "--reference="); ok {
			ref = v
		} else {
			rest = append(rest, a)
		}
	}
	return ref, rest
}

// buildResolver constructs a modeResolver and the file list from args.
func buildResolver(ref string, args []string) (modeResolver, []string, error) {
	if ref != "" {
		r, err := newReferenceResolver(ref)
		if err != nil {
			return nil, nil, err
		}
		return r, args, nil
	}
	if len(args) < 2 {
		return nil, nil, fmt.Errorf("chmod: missing operand after '%s'", args[0])
	}
	r, err := parseMode(args[0])
	if err != nil {
		return nil, nil, fmt.Errorf("chmod: invalid mode: '%s'", args[0])
	}
	return r, args[1:], nil
}

// applyToFiles applies the resolver to each file.
// R1.3: process multiple FILE arguments.
// R1.4: continue on error, exit 1 if any failed.
func applyToFiles(r modeResolver, files []string) int {
	exitCode := 0
	for _, f := range files {
		if err := applyMode(r, f); err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitCode = 1
		}
	}
	return exitCode
}

// applyMode reads the current mode and applies the resolver.
func applyMode(r modeResolver, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("chmod: cannot access '%s': %w", path, err)
	}
	newMode := r.resolve(info.Mode())
	if err := os.Chmod(path, newMode); err != nil {
		return fmt.Errorf("chmod: changing permissions of '%s': %w", path, err)
	}
	return nil
}

// --- Reference resolver (R3.2) ---

type referenceResolver struct {
	mode fs.FileMode
}

func newReferenceResolver(path string) (*referenceResolver, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf(
			"chmod: failed to get attributes of '%s': %w", path, err)
	}
	return &referenceResolver{mode: info.Mode().Perm() | specialBitsGo(info.Mode())}, nil
}

func (r *referenceResolver) resolve(_ fs.FileMode) fs.FileMode {
	return r.mode
}

// specialBitsGo extracts setuid/setgid/sticky from a Go FileMode.
func specialBitsGo(m fs.FileMode) fs.FileMode {
	return m & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
}

// --- Octal resolver (R1.1) ---

type octalResolver struct {
	mode fs.FileMode
}

func (r *octalResolver) resolve(_ fs.FileMode) fs.FileMode {
	return r.mode
}

// --- Symbolic resolver (R1.2, R3.1) ---

type symbolicResolver struct {
	clauses []symbolicClause
}

// symbolicClause represents one parsed clause like "u+rx".
type symbolicClause struct {
	who   uint32 // bitmask: 4=user, 2=group, 1=other
	op    byte   // '+', '-', '='
	rwx   uint32 // 3-bit rwx pattern (4=r, 2=w, 1=x)
	hasX  bool   // capital X: conditional execute
	hasS  bool   // s permission (setuid/setgid depending on who)
	hasT  bool   // t permission (sticky bit)
	noWho bool   // true when no ugoa specified (apply umask)
}

func (r *symbolicResolver) resolve(current fs.FileMode) fs.FileMode {
	mode := goModeToUnix(current)
	for _, c := range r.clauses {
		mode = applyClause(c, mode, current.IsDir())
	}
	return unixModeToGo(mode)
}

// goModeToUnix converts Go's fs.FileMode to a 12-bit Unix mode.
func goModeToUnix(m fs.FileMode) uint32 {
	mode := uint32(m.Perm())
	if m&os.ModeSetuid != 0 {
		mode |= 0o4000
	}
	if m&os.ModeSetgid != 0 {
		mode |= 0o2000
	}
	if m&os.ModeSticky != 0 {
		mode |= 0o1000
	}
	return mode
}

// unixModeToGo converts a 12-bit Unix mode to Go's fs.FileMode.
func unixModeToGo(mode uint32) fs.FileMode {
	m := fs.FileMode(mode & 0o777)
	if mode&0o4000 != 0 {
		m |= os.ModeSetuid
	}
	if mode&0o2000 != 0 {
		m |= os.ModeSetgid
	}
	if mode&0o1000 != 0 {
		m |= os.ModeSticky
	}
	return m
}

// applyClause applies a single symbolic clause to a 12-bit Unix mode.
func applyClause(c symbolicClause, mode uint32, isDir bool) uint32 {
	bits, mask := computeBitsAndMask(c, mode, isDir)
	switch c.op {
	case '+':
		mode |= bits
	case '-':
		mode &^= mask
	case '=':
		mode = (mode &^ mask) | bits
	}
	return mode
}

// computeBitsAndMask computes the bits to set and the mask to clear
// for a symbolic clause, expanding who selectors to their positions.
func computeBitsAndMask(c symbolicClause, mode uint32, isDir bool) (uint32, uint32) {
	rwx := c.rwx
	if c.hasX && (isDir || mode&0o111 != 0) {
		rwx |= 1 // add execute
	}
	umask := getUmask()
	who := c.who
	if c.noWho {
		who = 7 // all, but filtered by umask for bits
	}
	bits, mask := expandForWho(who, rwx, c.hasS, c.hasT)
	if c.noWho && c.op != '-' {
		bits &^= umaskToBits(umask)
	}
	return bits, mask
}

// expandForWho maps rwx bits and special flags to their correct
// positions based on who selectors.
func expandForWho(who, rwx uint32, hasS, hasT bool) (uint32, uint32) {
	var bits, mask uint32
	if who&4 != 0 { // user
		bits |= rwx << 6
		mask |= 0o700
		if hasS {
			bits |= 0o4000
			mask |= 0o4000
		}
	}
	if who&2 != 0 { // group
		bits |= rwx << 3
		mask |= 0o070
		if hasS {
			bits |= 0o2000
			mask |= 0o2000
		}
	}
	if who&1 != 0 { // other
		bits |= rwx
		mask |= 0o007
		if hasT {
			bits |= 0o1000
			mask |= 0o1000
		}
	}
	if hasT && who&1 == 0 {
		bits |= 0o1000
		mask |= 0o1000
	}
	return bits, mask
}

// umaskToBits converts a umask to the full 12-bit mask of bits to clear.
func umaskToBits(umask uint32) uint32 {
	return umask & 0o777
}

// getUmask retrieves the process umask.
func getUmask() uint32 {
	old := syscall.Umask(0)
	syscall.Umask(old)
	return uint32(old)
}

// --- Mode parsing ---

// parseMode determines whether the mode string is octal or symbolic.
func parseMode(s string) (modeResolver, error) {
	if r, ok := tryParseOctal(s); ok {
		return r, nil
	}
	return parseSymbolic(s)
}

// tryParseOctal parses an octal mode string like "755" or "0644".
// R1.1: octal mode parsing.
func tryParseOctal(s string) (*octalResolver, bool) {
	if len(s) == 0 {
		return nil, false
	}
	for _, c := range s {
		if c < '0' || c > '7' {
			return nil, false
		}
	}
	n, err := strconv.ParseUint(s, 8, 32)
	if err != nil || n > 0o7777 {
		return nil, false
	}
	return &octalResolver{mode: unixModeToGo(uint32(n))}, true
}

// parseSymbolic parses comma-separated symbolic mode clauses.
// R1.2: symbolic mode parsing with comma separation.
func parseSymbolic(s string) (*symbolicResolver, error) {
	parts := strings.Split(s, ",")
	var clauses []symbolicClause
	for _, part := range parts {
		parsed, err := parseOneSymbolic(part)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, parsed...)
	}
	return &symbolicResolver{clauses: clauses}, nil
}

// parseOneSymbolic parses a single symbolic clause like "u+x" or "go-w".
func parseOneSymbolic(s string) ([]symbolicClause, error) {
	i := 0
	who, noWho := parseWho(s, &i)
	var clauses []symbolicClause
	for i < len(s) {
		op := s[i]
		if op != '+' && op != '-' && op != '=' {
			return nil, fmt.Errorf("invalid operator '%c'", op)
		}
		i++
		rwx, hasX, hasS, hasT := parsePerms(s, &i)
		clauses = append(clauses, symbolicClause{
			who: who, op: op, rwx: rwx,
			hasX: hasX, hasS: hasS, hasT: hasT, noWho: noWho,
		})
	}
	if len(clauses) == 0 {
		return nil, fmt.Errorf("invalid mode '%s'", s)
	}
	return clauses, nil
}

// parseWho parses the who portion (ugoa) and advances i.
func parseWho(s string, i *int) (uint32, bool) {
	var who uint32
	for *i < len(s) {
		switch s[*i] {
		case 'u':
			who |= 4
		case 'g':
			who |= 2
		case 'o':
			who |= 1
		case 'a':
			who = 7
		default:
			if who == 0 {
				return 0, true
			}
			return who, false
		}
		*i++
	}
	if who == 0 {
		return 0, true
	}
	return who, false
}

// parsePerms parses permission characters after the operator.
func parsePerms(s string, i *int) (uint32, bool, bool, bool) {
	var rwx uint32
	var hasX, hasS, hasT bool
	for *i < len(s) {
		switch s[*i] {
		case 'r':
			rwx |= 4
		case 'w':
			rwx |= 2
		case 'x':
			rwx |= 1
		case 'X':
			hasX = true
		case 's':
			hasS = true
		case 't':
			hasT = true
		default:
			return rwx, hasX, hasS, hasT
		}
		*i++
	}
	return rwx, hasX, hasS, hasT
}
