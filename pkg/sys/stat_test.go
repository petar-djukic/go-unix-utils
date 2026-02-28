// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// stat_test.go contains unit tests for ExtractMetadata, verifying ModTime and
// Blocks on real temporary files, and error handling when Sys() returns a
// non-Stat_t type.
//
// Tests: prd002-sys R2, R2.3.
package sys

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestExtractMetadata_RealFile(t *testing.T) {
	// ExtractMetadata on a real temporary file must return FileMetadata where
	// ModTime is within 5 seconds of time.Now() and Blocks is non-negative (R2, R2.3).
	before := time.Now().Add(-5 * time.Second)

	f, err := os.CreateTemp(t.TempDir(), "metadata-test-*")
	if err != nil {
		t.Fatalf("os.CreateTemp() failed: %v", err)
	}
	// Write some content so blocks are allocated.
	if _, err := f.WriteString("test content for metadata extraction\n"); err != nil {
		f.Close()
		t.Fatalf("writing to temp file: %v", err)
	}
	f.Close()

	after := time.Now().Add(5 * time.Second)

	info, err := os.Stat(f.Name())
	if err != nil {
		t.Fatalf("os.Stat(%s) failed: %v", f.Name(), err)
	}

	meta, err := ExtractMetadata(info)
	if err != nil {
		t.Fatalf("ExtractMetadata() returned error: %v", err)
	}

	t.Run("modtime_within_tolerance", func(t *testing.T) {
		if meta.ModTime.Before(before) {
			t.Errorf("ExtractMetadata().ModTime = %v, want after %v", meta.ModTime, before)
		}
		if meta.ModTime.After(after) {
			t.Errorf("ExtractMetadata().ModTime = %v, want before %v", meta.ModTime, after)
		}
	})

	t.Run("blocks_non_negative", func(t *testing.T) {
		if meta.Blocks < 0 {
			t.Errorf("ExtractMetadata().Blocks = %d, want non-negative", meta.Blocks)
		}
	})
}

func TestExtractMetadata_EmptyFile(t *testing.T) {
	// ExtractMetadata on an empty file must also succeed (R2.3).
	f, err := os.CreateTemp(t.TempDir(), "empty-metadata-*")
	if err != nil {
		t.Fatalf("os.CreateTemp() failed: %v", err)
	}
	f.Close()

	info, err := os.Stat(f.Name())
	if err != nil {
		t.Fatalf("os.Stat(%s) failed: %v", f.Name(), err)
	}

	meta, err := ExtractMetadata(info)
	if err != nil {
		t.Fatalf("ExtractMetadata() returned error: %v", err)
	}

	if meta.Blocks < 0 {
		t.Errorf("ExtractMetadata().Blocks = %d, want non-negative for empty file", meta.Blocks)
	}
}

func TestExtractMetadata_Directory(t *testing.T) {
	// ExtractMetadata on a directory must succeed (R2.3).
	dir := t.TempDir()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat(%s) failed: %v", dir, err)
	}

	meta, err := ExtractMetadata(info)
	if err != nil {
		t.Fatalf("ExtractMetadata() returned error: %v", err)
	}

	if meta.Blocks < 0 {
		t.Errorf("ExtractMetadata().Blocks = %d, want non-negative for directory", meta.Blocks)
	}
}

// fakeFileInfo is a minimal os.FileInfo whose Sys() returns a non-Stat_t value,
// used to verify ExtractMetadata error handling.
type fakeFileInfo struct {
	os.FileInfo
}

func (f fakeFileInfo) Sys() interface{} {
	return "not a *syscall.Stat_t"
}

func (f fakeFileInfo) Name() string               { return "fake" }
func (f fakeFileInfo) Size() int64                 { return 0 }
func (f fakeFileInfo) Mode() os.FileMode           { return 0 }
func (f fakeFileInfo) ModTime() time.Time          { return time.Time{} }
func (f fakeFileInfo) IsDir() bool                 { return false }
func (f fakeFileInfo) String() string              { return fmt.Sprintf("fakeFileInfo{%s}", f.Name()) }

func TestExtractMetadata_NonStatT(t *testing.T) {
	// ExtractMetadata must return an error when Sys() is not *syscall.Stat_t (R2.3).
	fake := fakeFileInfo{}
	_, err := ExtractMetadata(fake)
	if err == nil {
		t.Error("ExtractMetadata(fakeFileInfo) = nil error, want error for non-Stat_t Sys()")
	}
}
