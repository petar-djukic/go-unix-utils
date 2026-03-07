// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"reflect"
	"testing"
)

func TestColumns_Empty(t *testing.T) {
	t.Parallel()
	// R1.1: empty input returns nil.
	got := Columns(nil, 80)
	if got != nil {
		t.Errorf("Columns(nil, 80) = %v, want nil", got)
	}
}

func TestColumns_EmptySlice(t *testing.T) {
	t.Parallel()
	got := Columns([]string{}, 80)
	if got != nil {
		t.Errorf("Columns([], 80) = %v, want nil", got)
	}
}

func TestColumns_SingleEntry(t *testing.T) {
	t.Parallel()
	// Single entry always produces one row with one column.
	got := Columns([]string{"hello"}, 80)
	want := [][]string{{"hello"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns single entry:\ngot  %v\nwant %v", got, want)
	}
}

func TestColumns_SingleColumn(t *testing.T) {
	t.Parallel()
	// R1.1: entries too wide to fit more than one column fall back to single-column.
	entries := []string{"very-long-filename-a", "very-long-filename-b", "very-long-filename-c"}
	got := Columns(entries, 25)
	want := [][]string{
		{"very-long-filename-a"},
		{"very-long-filename-b"},
		{"very-long-filename-c"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns single column:\ngot  %v\nwant %v", got, want)
	}
}

func TestColumns_EntryWiderThanTermWidth(t *testing.T) {
	t.Parallel()
	// When entries exceed termWidth, still produces single-column output.
	entries := []string{"abcdefghij", "klmnopqrst"}
	got := Columns(entries, 5)
	want := [][]string{
		{"abcdefghij"},
		{"klmnopqrst"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns entry wider than termWidth:\ngot  %v\nwant %v", got, want)
	}
}

func TestColumns_MultiColumn(t *testing.T) {
	t.Parallel()
	// R1.1: entries arranged top-to-bottom then left-to-right.
	entries := []string{"a", "b", "c", "d", "e", "f"}
	got := Columns(entries, 12)

	// numCols=5, numRows=2: col0=[a,b] col1=[c,d] col2=[e,f] col3,col4 empty.
	// Rows: [a,c,e], [b,d,f].
	want := [][]string{
		{"a", "c", "e"},
		{"b", "d", "f"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns multi:\ngot  %v\nwant %v", got, want)
	}
}

func TestColumns_TopToBottomLeftToRight(t *testing.T) {
	t.Parallel()
	// R1.4: verify top-to-bottom, left-to-right ordering with varying widths.
	entries := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta"}
	got := Columns(entries, 30)

	// 4 cols, numRows=2: col0=[alpha,beta](5), col1=[gamma,delta](5),
	//   col2=[epsilon,zeta](7), col3=[eta,theta](5).
	//   total = 5+2+5+2+7+2+5 = 28 <= 30.
	want := [][]string{
		{"alpha", "gamma", "epsilon", "eta"},
		{"beta", "delta", "zeta", "theta"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns 8 entries:\ngot  %v\nwant %v", got, want)
	}
}

func TestColumns_UnevenDistribution(t *testing.T) {
	t.Parallel()
	// 5 entries in 2 columns: 3 rows, last column has 2 entries.
	entries := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	got := Columns(entries, 10)
	want := [][]string{
		{"aaa", "ddd"},
		{"bbb", "eee"},
		{"ccc"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns uneven:\ngot  %v\nwant %v", got, want)
	}
}

func TestColumns_NarrowTermWidth(t *testing.T) {
	t.Parallel()
	// termWidth=1 forces single-column for any multi-char entry.
	entries := []string{"ab", "cd"}
	got := Columns(entries, 1)
	want := [][]string{
		{"ab"},
		{"cd"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns narrow termWidth:\ngot  %v\nwant %v", got, want)
	}
}

func TestColumns_AllFitOneRow(t *testing.T) {
	t.Parallel()
	// Wide terminal: all entries fit in a single row.
	entries := []string{"a", "b", "c"}
	got := Columns(entries, 80)
	// numCols=3, numRows=1: each entry is its own column.
	want := [][]string{
		{"a", "b", "c"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns all fit one row:\ngot  %v\nwant %v", got, want)
	}
}

func TestColumns_TwoEntries(t *testing.T) {
	t.Parallel()
	// Two entries that fit side by side.
	entries := []string{"foo", "bar"}
	got := Columns(entries, 10)
	// 2 cols, 1 row: total = 3+2+3=8 <= 10.
	want := [][]string{
		{"foo", "bar"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns two entries:\ngot  %v\nwant %v", got, want)
	}
}
