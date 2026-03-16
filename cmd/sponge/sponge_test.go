// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sponge against sponge (moreutils).
// Implements prd007-sponge R1.1-R1.5, R2.1-R2.5, R3.1-R3.3, R4.1-R4.3 test coverage.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.2: passthrough small stdin to stdout (no filename).
		{
			Name:  "R1.2_passthrough_small",
			Stdin: []byte("hello\n"),
		},
		// R1.2: passthrough empty stdin.
		{
			Name:  "R1.2_passthrough_empty",
			Stdin: []byte(""),
		},
		// R1.2: passthrough multi-line.
		{
			Name:  "R1.2_passthrough_multiline",
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		// R1.2: passthrough no trailing newline.
		{
			Name:  "R1.2_passthrough_no_trailing_newline",
			Stdin: []byte("abc"),
		},
		// R1.2: passthrough binary data.
		{
			Name:  "R1.2_passthrough_binary",
			Stdin: []byte{0x00, 0x01, 0xFF, 0xFE, '\n'},
		},
		// R1.4: success exit 0.
		{
			Name:     "R1.4_success_exit_0",
			Stdin:    []byte("data\n"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestFileOutput verifies that sponge writes stdin content to the named file.
// These are Go-binary-only tests because the differential harness runs both
// binaries in the same WorkDir, and file-output tests need isolated state.
func TestFileOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// R1.1, R1.3: write stdin to a new file.
	t.Run("R1.3_write_new_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "out.txt")
		input := []byte("hello\nworld\n")

		runSponge(t, goBin, dir, input, outFile)

		got := readFile(t, outFile)
		if !bytes.Equal(got, input) {
			t.Errorf("expected %q, got %q", input, got)
		}
	})

	// R1.3: write stdin to an existing file (truncates).
	t.Run("R1.3_truncate_existing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "existing.txt")
		writeFile(t, outFile, "old content that is longer\n")
		input := []byte("new\n")

		runSponge(t, goBin, dir, input, outFile)

		got := readFile(t, outFile)
		if !bytes.Equal(got, input) {
			t.Errorf("expected %q, got %q", input, got)
		}
	})

	// R1.1: soak-before-write contract — reading from and writing to the
	// same file must not produce empty output.
	t.Run("R1.1_soak_same_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "data.txt")
		original := []byte("original content\n")
		writeFile(t, outFile, string(original))

		// cat data.txt | sponge data.txt — must produce original content.
		catBin, err := exec.LookPath("cat")
		if err != nil {
			t.Skip("cat not found in PATH")
		}

		bashBin, err := exec.LookPath("bash")
		if err != nil {
			t.Skip("bash not found in PATH")
		}

		cmd := exec.Command(bashBin, "-c",
			catBin+" "+outFile+" | "+goBin+" "+outFile)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("soak-before-write test failed: %v\noutput: %s", err, out)
		}

		got := readFile(t, outFile)
		if !bytes.Equal(got, original) {
			t.Errorf("soak-before-write violated: expected %q, got %q", original, got)
		}
	})

	// R1.3: write empty stdin to file.
	t.Run("R1.3_empty_stdin", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "empty.txt")

		runSponge(t, goBin, dir, []byte{}, outFile)

		got := readFile(t, outFile)
		if len(got) != 0 {
			t.Errorf("expected empty file, got %q", got)
		}
	})

	// R1.4: exit non-zero on unwritable path.
	t.Run("R1.4_unwritable_path", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Create a read-only directory so the file cannot be created.
		roDir := filepath.Join(dir, "readonly")
		if err := os.Mkdir(roDir, 0o555); err != nil {
			t.Fatalf("creating read-only dir: %v", err)
		}
		outFile := filepath.Join(roDir, "out.txt")

		cmd := exec.Command(goBin, outFile)
		cmd.Stdin = bytes.NewReader([]byte("data\n"))
		cmd.Dir = dir
		err := cmd.Run()
		if err == nil {
			t.Error("expected non-zero exit for unwritable path, got exit 0")
		}
	})

	// R3.1: -a appends stdin to existing file.
	t.Run("R3.1_append_existing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "existing.txt")
		writeFile(t, outFile, "original\n")
		input := []byte("appended\n")

		cmd := exec.Command(goBin, "-a", outFile)
		cmd.Stdin = bytes.NewReader(input)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("sponge -a failed: %v\noutput: %s", err, out)
		}

		got := readFile(t, outFile)
		want := []byte("original\nappended\n")
		if !bytes.Equal(got, want) {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	// R3.2: -a with non-existing file creates new file.
	t.Run("R3.2_append_new_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "newfile.txt")
		input := []byte("new content\n")

		cmd := exec.Command(goBin, "-a", outFile)
		cmd.Stdin = bytes.NewReader(input)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("sponge -a new file failed: %v\noutput: %s", err, out)
		}

		got := readFile(t, outFile)
		if !bytes.Equal(got, input) {
			t.Errorf("expected %q, got %q", input, got)
		}
	})

	// R2.3: new file gets default permissions (0666 before umask).
	t.Run("R2.3_new_file_permissions", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "perms.txt")
		input := []byte("data\n")

		runSponge(t, goBin, dir, input, outFile)

		info, err := os.Stat(outFile)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		// File should be created; exact perms depend on umask, but it must exist.
		if !info.Mode().IsRegular() {
			t.Errorf("expected regular file, got mode %v", info.Mode())
		}
	})

	// R3.1: -a appends multiple times.
	t.Run("R3.1_append_multiple", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "multi.txt")
		writeFile(t, outFile, "first\n")

		// First append.
		cmd := exec.Command(goBin, "-a", outFile)
		cmd.Stdin = bytes.NewReader([]byte("second\n"))
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("first append failed: %v\noutput: %s", err, out)
		}

		// Second append.
		cmd = exec.Command(goBin, "-a", outFile)
		cmd.Stdin = bytes.NewReader([]byte("third\n"))
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("second append failed: %v\noutput: %s", err, out)
		}

		got := readFile(t, outFile)
		want := []byte("first\nsecond\nthird\n")
		if !bytes.Equal(got, want) {
			t.Errorf("expected %q, got %q", want, got)
		}
	})
}

// TestAppendDiff runs differential tests for append mode against the reference binary.
func TestAppendDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	// R3.1: -a appends to existing file — compare Go and reference outputs.
	t.Run("R3.1_append_diff", func(t *testing.T) {
		t.Parallel()

		input := []byte("appended line\n")
		original := "existing content\n"

		for _, label := range []string{"go", "ref"} {
			t.Run(label, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				outFile := filepath.Join(dir, "out.txt")
				writeFile(t, outFile, original)

				bin := goBin
				if label == "ref" {
					bin = refBin
				}

				cmd := exec.Command(bin, "-a", outFile)
				cmd.Stdin = bytes.NewReader(input)
				cmd.Dir = dir
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("sponge -a failed: %v\noutput: %s", err, out)
				}

				got := readFile(t, outFile)
				want := []byte(original + string(input))
				if !bytes.Equal(got, want) {
					t.Errorf("expected %q, got %q", want, got)
				}
			})
		}
	})

	// R3.2: -a with non-existing file — both should create it.
	t.Run("R3.2_append_new_diff", func(t *testing.T) {
		t.Parallel()

		input := []byte("new file content\n")

		for _, label := range []string{"go", "ref"} {
			t.Run(label, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				outFile := filepath.Join(dir, "newout.txt")

				bin := goBin
				if label == "ref" {
					bin = refBin
				}

				cmd := exec.Command(bin, "-a", outFile)
				cmd.Stdin = bytes.NewReader(input)
				cmd.Dir = dir
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("sponge -a new file failed: %v\noutput: %s", err, out)
				}

				got := readFile(t, outFile)
				if !bytes.Equal(got, input) {
					t.Errorf("expected %q, got %q", input, got)
				}
			})
		}
	})
}

// TestSymlinkOutput verifies R2.4: sponge writes through symlinks instead of
// replacing them. Runs both Go and reference binaries when available.
func TestSymlinkOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, _ := exec.LookPath("sponge")

	for _, tc := range []struct {
		name  string
		label string
		bin   string
	}{
		{"R2.4_symlink_go", "go", goBin},
		{"R2.4_symlink_ref", "ref", refBin},
	} {
		if tc.bin == "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			target := filepath.Join(dir, "target.txt")
			writeFile(t, target, "old target content\n")
			link := filepath.Join(dir, "link.txt")
			if err := os.Symlink(target, link); err != nil {
				t.Fatalf("creating symlink: %v", err)
			}

			input := []byte("new content via symlink\n")
			cmd := exec.Command(tc.bin, link)
			cmd.Stdin = bytes.NewReader(input)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sponge via symlink failed: %v\noutput: %s", err, out)
			}

			// R2.4: Symlink must still exist (not replaced by regular file).
			info, err := os.Lstat(link)
			if err != nil {
				t.Fatalf("lstat link: %v", err)
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Error("symlink was replaced by regular file")
			}

			// Target must have the new content.
			got := readFile(t, target)
			if !bytes.Equal(got, input) {
				t.Errorf("expected %q, got %q", input, got)
			}
		})
	}
}

// TestSymlinkAppend verifies R3.2 with symlink output in append mode.
// R3.2: -a applies only when the output file exists and is a regular file.
// A symlink is not a regular file (per lstat), so -a behaves like default
// mode — stdin content replaces the target's content.
func TestSymlinkAppend(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, _ := exec.LookPath("sponge")

	for _, tc := range []struct {
		name string
		bin  string
	}{
		{"R3.2_symlink_append_go", goBin},
		{"R3.2_symlink_append_ref", refBin},
	} {
		if tc.bin == "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			target := filepath.Join(dir, "target.txt")
			writeFile(t, target, "original\n")
			link := filepath.Join(dir, "link.txt")
			if err := os.Symlink(target, link); err != nil {
				t.Fatalf("creating symlink: %v", err)
			}

			input := []byte("new content\n")
			cmd := exec.Command(tc.bin, "-a", link)
			cmd.Stdin = bytes.NewReader(input)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sponge -a via symlink failed: %v\noutput: %s", err, out)
			}

			// R3.2: Symlink is not a regular file per lstat, so -a falls
			// through to default mode — only stdin content is written.
			got := readFile(t, target)
			if !bytes.Equal(got, input) {
				t.Errorf("expected %q, got %q", input, got)
			}
		})
	}
}

// TestAtomicWrite verifies R2.5: the output file is never in a
// partially-written state. We verify by overwriting a larger file with
// smaller content and confirming no leftover bytes remain.
func TestAtomicWrite(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, _ := exec.LookPath("sponge")

	for _, tc := range []struct {
		name string
		bin  string
	}{
		{"R2.5_atomic_overwrite_go", goBin},
		{"R2.5_atomic_overwrite_ref", refBin},
	} {
		if tc.bin == "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			outFile := filepath.Join(dir, "out.txt")
			// Write a large initial file.
			writeFile(t, outFile, "this is a much longer string of old content\n")
			// Overwrite with smaller content.
			input := []byte("short\n")

			cmd := exec.Command(tc.bin, outFile)
			cmd.Stdin = bytes.NewReader(input)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sponge failed: %v\noutput: %s", err, out)
			}

			got := readFile(t, outFile)
			if !bytes.Equal(got, input) {
				t.Errorf("expected %q, got %q (leftover bytes from old content)", input, got)
			}
		})
	}
}

// TestAppendEmptyStdin verifies R3.1 edge case: appending empty stdin
// preserves the original file content unchanged.
func TestAppendEmptyStdin(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, _ := exec.LookPath("sponge")

	for _, tc := range []struct {
		name string
		bin  string
	}{
		{"R3.1_append_empty_go", goBin},
		{"R3.1_append_empty_ref", refBin},
	} {
		if tc.bin == "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			outFile := filepath.Join(dir, "existing.txt")
			original := "keep this content\n"
			writeFile(t, outFile, original)

			cmd := exec.Command(tc.bin, "-a", outFile)
			cmd.Stdin = bytes.NewReader([]byte{})
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sponge -a empty stdin failed: %v\noutput: %s", err, out)
			}

			got := readFile(t, outFile)
			if !bytes.Equal(got, []byte(original)) {
				t.Errorf("expected %q, got %q", original, got)
			}
		})
	}
}

// TestAppendAtomic verifies R3.3: append mode uses temp-file-then-rename
// approach. The original file content is copied into the temp file first,
// then stdin is appended, then the temp file is renamed over the original.
// We verify by checking that the result is [original][stdin] and that the
// file permissions are preserved (indicating the atomic path was used).
func TestAppendAtomic(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, _ := exec.LookPath("sponge")

	for _, tc := range []struct {
		name string
		bin  string
	}{
		{"R3.3_append_atomic_go", goBin},
		{"R3.3_append_atomic_ref", refBin},
	} {
		if tc.bin == "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			outFile := filepath.Join(dir, "atomic.txt")
			original := "original content here\n"
			writeFile(t, outFile, original)
			// Set a distinctive permission to verify preservation.
			if err := os.Chmod(outFile, 0o640); err != nil {
				t.Fatalf("chmod: %v", err)
			}

			input := []byte("appended via atomic\n")
			cmd := exec.Command(tc.bin, "-a", outFile)
			cmd.Stdin = bytes.NewReader(input)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sponge -a failed: %v\noutput: %s", err, out)
			}

			// R3.3: Result must be [original][stdin].
			got := readFile(t, outFile)
			want := []byte(original + string(input))
			if !bytes.Equal(got, want) {
				t.Errorf("expected %q, got %q", want, got)
			}

			// R2.3: Permissions must be preserved through the atomic path.
			info, err := os.Stat(outFile)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			if info.Mode().Perm() != 0o640 {
				t.Errorf("expected mode 0640, got %04o", info.Mode().Perm())
			}
		})
	}
}

// TestAppendLargeContent verifies R3.3 with larger content to exercise
// the temp file write path more thoroughly.
func TestAppendLargeContent(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, _ := exec.LookPath("sponge")

	// Build a ~100KB original and ~100KB stdin to ensure the temp file
	// path handles non-trivial sizes.
	originalData := bytes.Repeat([]byte("original line of data\n"), 5000)
	stdinData := bytes.Repeat([]byte("appended line of data\n"), 5000)

	for _, tc := range []struct {
		name string
		bin  string
	}{
		{"R3.3_append_large_go", goBin},
		{"R3.3_append_large_ref", refBin},
	} {
		if tc.bin == "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			outFile := filepath.Join(dir, "large.txt")
			if err := os.WriteFile(outFile, originalData, 0o644); err != nil {
				t.Fatalf("writing file: %v", err)
			}

			cmd := exec.Command(tc.bin, "-a", outFile)
			cmd.Stdin = bytes.NewReader(stdinData)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sponge -a large failed: %v\noutput: %s", err, out)
			}

			got := readFile(t, outFile)
			want := append(originalData, stdinData...)
			if !bytes.Equal(got, want) {
				t.Errorf("large append mismatch: got %d bytes, want %d bytes", len(got), len(want))
			}
		})
	}
}

// TestPassthroughDiff runs differential tests for R4.1-R4.3: passthrough
// mode (no output filename) writes buffered stdin to stdout.
func TestPassthroughDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.1, R4.3: small in-memory buffer written directly to stdout.
		{
			Name:  "R4.1_passthrough_hello",
			Stdin: []byte("hello world\n"),
		},
		// R4.1, R4.3: passthrough with multiple lines.
		{
			Name:  "R4.1_passthrough_multiline",
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		// R4.3: empty input passthrough.
		{
			Name:  "R4.3_passthrough_empty",
			Stdin: []byte{},
		},
		// R4.3: passthrough binary data with no trailing newline.
		{
			Name:  "R4.3_passthrough_binary_no_newline",
			Stdin: []byte{0x00, 0xFF, 0x80, 0x7F},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestPassthroughLarge verifies R4.1 with a larger payload to exercise
// the in-memory passthrough path (R4.3) with substantial data.
func TestPassthroughLarge(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, _ := exec.LookPath("sponge")

	// ~200KB of data to passthrough.
	largeData := bytes.Repeat([]byte("passthrough line\n"), 12000)

	for _, tc := range []struct {
		name string
		bin  string
	}{
		{"R4.1_passthrough_large_go", goBin},
		{"R4.1_passthrough_large_ref", refBin},
	} {
		if tc.bin == "" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(tc.bin)
			cmd.Stdin = bytes.NewReader(largeData)
			got, err := cmd.Output()
			if err != nil {
				t.Fatalf("sponge passthrough failed: %v", err)
			}
			if !bytes.Equal(got, largeData) {
				t.Errorf("passthrough mismatch: got %d bytes, want %d bytes", len(got), len(largeData))
			}
		})
	}
}

// TestPermissionPreservation verifies R2.3 with R2.4: permissions are preserved
// when writing to an existing file, including through symlinks.
func TestPermissionPreservation(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// R2.3: Regular file permission preservation.
	t.Run("R2.3_preserve_mode", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "perms.txt")
		writeFile(t, outFile, "old\n")
		if err := os.Chmod(outFile, 0o755); err != nil {
			t.Fatalf("chmod: %v", err)
		}

		runSponge(t, goBin, dir, []byte("new\n"), outFile)

		info, err := os.Stat(outFile)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Errorf("expected mode 0755, got %04o", info.Mode().Perm())
		}
	})

	// R2.4: Permission preservation through symlink.
	t.Run("R2.4_symlink_preserve_mode", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target := filepath.Join(dir, "target.txt")
		writeFile(t, target, "old\n")
		if err := os.Chmod(target, 0o700); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		link := filepath.Join(dir, "link.txt")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}

		cmd := exec.Command(goBin, link)
		cmd.Stdin = bytes.NewReader([]byte("new\n"))
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("sponge via symlink failed: %v\noutput: %s", err, out)
		}

		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Errorf("expected mode 0700, got %04o", info.Mode().Perm())
		}
	})
}

// runSponge runs the sponge binary with the given stdin and output file argument.
func runSponge(t *testing.T, bin, dir string, stdin []byte, outFile string) {
	t.Helper()
	cmd := exec.Command(bin, outFile)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sponge failed: %v\noutput: %s", err, out)
	}
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing file %s: %v", path, err)
	}
}

// readFile reads and returns the contents of a file.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file %s: %v", path, err)
	}
	return data
}
