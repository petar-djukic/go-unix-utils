// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"reflect"
	"testing"
)

func TestColumns_Empty(t *testing.T) {
	t.Parallel()
	got := Columns(nil, 80)
	if got != nil {
		t.Errorf("Columns(nil, 80) = %v, want nil", got)
	}
}

func TestColumns_SingleColumn(t *testing.T) {
	t.Parallel()
	// Entries too wide to fit more than one column.
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

func TestColumns_MultiColumn(t *testing.T) {
	t.Parallel()
	// AC7: entries arranged top-to-bottom then left-to-right.
	entries := []string{"a", "b", "c", "d", "e", "f"}
	// Each entry is 1 char wide. With gap=2, each column takes 3 chars except last.
	// 6 entries, termWidth=12: 4 cols × 2 rows = (3+3+3+1)=10 fits in 12.
	got := Columns(entries, 12)

	// 4 columns, 2 rows: col0=[a,b], col1=[c,d], col2=[e,f] -- wait that's 3 cols.
	// Actually with 6 entries and 6 cols: 6*1 + 5*2 = 16 > 12. 5 cols: 5 + 4*2 = 13 > 12.
	// 4 cols: 2 rows. col widths: col0=max(a,b)=1, col1=max(c,d)=1, col2=max(e,f)=1, col3 empty or not.
	// Wait: 4 cols, 2 rows = 8 slots, 6 entries. col0=[0,1]=a,b. col1=[2,3]=c,d. col2=[4,5]=e,f. col3 empty.
	// That's only 3 columns needed. Let me recalculate:
	// 6 cols: each col has 1 row. total = 6*1 + 5*2 = 16 > 12. No.
	// 5 cols: 2 rows. 5*2=10 slots. col0=[a,b] col1=[c,d] col2=[e,f] col3=[?,?] col4=[?,?]. only 6 entries so col3 has index 6,7 which are >=n.
	// Actually col3 row0 = 3*2+0=6 >= 6, so col3 is empty. So effectively 3 columns.
	// With numCols=5, numRows=2: col widths computed, but cols 3,4 have width 0.
	// total = 1+2+1+2+1+2+0+2+0 = 1+1+1+0+0 + 4*2 = 3+8=11? No.
	// totalWidth = colWidths[0]+gap + colWidths[1]+gap + colWidths[2]+gap + colWidths[3]+gap + colWidths[4]
	// = 1+2+1+2+1+2+0+2+0 = 11. Fits in 12. So 5 cols is accepted but cols 3,4 are empty.
	// Rows: row0=[a,c,e], row1=[b,d,f]. That's 3 entries per row.

	// Actually let's try numCols=6: numRows=1. total=6*1+5*2=16>12. No.
	// numCols=5: numRows=2. Indices: col0=[0,1] col1=[2,3] col2=[4,5] col3=[6,7] col4=[8,9]. Only 0-5 valid.
	// So row0: entries[0]=a, entries[2]=c, entries[4]=e (col3 idx=6 >=n, break). Row = [a,c,e].
	// row1: entries[1]=b, entries[3]=d, entries[5]=f (col3 idx=7 >=n, break). Row = [b,d,f].
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
	// Verify top-to-bottom, left-to-right ordering with varying widths.
	entries := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta"}
	// Longest is "epsilon" = 7 chars. With gap=2, two columns need 7+2+7=16.
	// Three columns: need 3 cols of per-col max. Let's use termWidth=30.
	got := Columns(entries, 30)

	// 8 entries, try 8 cols: numRows=1. total = sum of widths + 7*2.
	// Widths: 5+4+5+5+7+4+3+5 = 38 + 14 = 52 > 30.
	// 4 cols: numRows=2. col0=[alpha,beta](max=5), col1=[gamma,delta](max=5),
	//   col2=[epsilon,zeta](max=7), col3=[eta,theta](max=5).
	//   total = 5+2+5+2+7+2+5 = 28 <= 30. Fits!
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
	// 2 cols: numRows=3. col0=[aaa,bbb,ccc](3), col1=[ddd,eee](2).
	// Width: 3+2+3=8. termWidth=10 fits.
	// 3 cols: numRows=2. col0=[aaa,bbb] col1=[ccc,ddd] col2=[eee].
	// Width: 3+2+3+2+3=13 > 10. No.
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
