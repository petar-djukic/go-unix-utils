// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd067-split R2.3: chunk-based splitting modes.
// Supports -n N (bytes), -n l/N (lines), -n r/N (round-robin).
package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
)

// splitByChunksBytes splits input into n pieces of roughly equal byte
// size. R2.3: -n N mode. First (totalSize%n) chunks get one extra byte.
func splitByChunksBytes(
	f *os.File, n int64, namer *fileNamer,
) error {
	totalSize, r, err := prepareChunkInput(f)
	if err != nil {
		return err
	}
	return writeByteChunkPieces(r, totalSize, n, namer)
}

// prepareChunkInput determines total size and returns a sequential
// reader. For stdin, all data is buffered in memory.
func prepareChunkInput(
	f *os.File,
) (int64, io.Reader, error) {
	if f == os.Stdin {
		data, err := io.ReadAll(f)
		if err != nil {
			return 0, nil, err
		}
		return int64(len(data)), bytes.NewReader(data), nil
	}
	info, err := f.Stat()
	if err != nil {
		return 0, nil, err
	}
	return info.Size(), f, nil
}

// writeByteChunkPieces writes n pieces from a sequential reader.
func writeByteChunkPieces(
	r io.Reader, totalSize, n int64, namer *fileNamer,
) error {
	chunkSize := totalSize / n
	remainder := totalSize % n

	for i := int64(0); i < n; i++ {
		size := chunkSize
		if i < remainder {
			size++
		}
		if err := writeOnePiece(r, size, namer); err != nil {
			return err
		}
	}
	return nil
}

// writeOnePiece creates one output piece of the given byte size.
func writeOnePiece(
	r io.Reader, size int64, namer *fileNamer,
) error {
	w, err := namer.next()
	if err != nil {
		return err
	}
	defer w.Close()
	if size > 0 {
		if _, cerr := io.CopyN(w, r, size); cerr != nil && cerr != io.EOF {
			return cerr
		}
	}
	return nil
}

// splitByChunksLines splits input into n pieces by line count,
// distributing lines as evenly as possible.
// R2.3: -n l/N mode.
func splitByChunksLines(
	r io.Reader, n int64, namer *fileNamer,
) error {
	lines, err := readAllLines(r)
	if err != nil {
		return err
	}
	return distributeLines(lines, n, namer)
}

// distributeLines writes lines to n chunks. First (total%n) chunks
// get one extra line.
func distributeLines(
	lines [][]byte, n int64, namer *fileNamer,
) error {
	totalLines := int64(len(lines))
	linesPerChunk := totalLines / n
	remainder := totalLines % n

	var offset int64
	for i := int64(0); i < n; i++ {
		count := linesPerChunk
		if i < remainder {
			count++
		}
		if err := writeLineChunk(lines, offset, count, namer); err != nil {
			return err
		}
		offset += count
	}
	return nil
}

// writeLineChunk writes count lines starting at offset to one piece.
func writeLineChunk(
	lines [][]byte, offset, count int64, namer *fileNamer,
) error {
	w, err := namer.next()
	if err != nil {
		return err
	}
	defer w.Close()
	for j := int64(0); j < count; j++ {
		if _, werr := w.Write(lines[offset+j]); werr != nil {
			return werr
		}
	}
	return nil
}

// readAllLines reads all lines from a reader, preserving newlines.
func readAllLines(r io.Reader) ([][]byte, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	var lines [][]byte
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			cp := make([]byte, len(line))
			copy(cp, line)
			lines = append(lines, cp)
		}
		if err != nil {
			if err != io.EOF {
				return nil, err
			}
			break
		}
	}
	return lines, nil
}

// splitByChunksRoundRobin distributes lines round-robin to n files.
// R2.3: -n r/N mode.
func splitByChunksRoundRobin(
	r io.Reader, n int64, namer *fileNamer,
) error {
	writers, err := openNWriters(n, namer)
	if err != nil {
		return err
	}
	defer closeAllWriters(writers)

	br := bufio.NewReaderSize(r, 64*1024)
	var idx int64
	for {
		line, rerr := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := writers[idx%n].Write(line); werr != nil {
				return werr
			}
			idx++
		}
		if rerr != nil {
			if rerr != io.EOF {
				return rerr
			}
			break
		}
	}
	return nil
}

// openNWriters opens n output writers upfront for round-robin.
func openNWriters(
	n int64, namer *fileNamer,
) ([]io.WriteCloser, error) {
	writers := make([]io.WriteCloser, n)
	for i := int64(0); i < n; i++ {
		w, err := namer.next()
		if err != nil {
			closeAllWriters(writers[:i])
			return nil, err
		}
		writers[i] = w
	}
	return writers, nil
}

// closeAllWriters closes all non-nil writers.
func closeAllWriters(writers []io.WriteCloser) {
	for _, w := range writers {
		if w != nil {
			w.Close() // best-effort close
		}
	}
}
