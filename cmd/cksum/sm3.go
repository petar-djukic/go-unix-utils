// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// sm3.go implements the SM3 cryptographic hash function (GB/T 32905-2016).
//
// R2.1: SM3 algorithm support for --algorithm=sm3.
package main

import (
	"encoding/binary"
	"hash"
	"math/bits"
)

const (
	sm3DigestSize = 32
	sm3BlockSize  = 64
)

// sm3IV contains the initial hash values for SM3.
var sm3IV = [8]uint32{
	0x7380166f, 0x4914b2b9, 0x172442d7, 0xda8a0600,
	0xa96f30bc, 0x163138aa, 0xe38dee4d, 0xb0fb0e4e,
}

// sm3Digest implements hash.Hash for the SM3 algorithm.
type sm3Digest struct {
	h   [8]uint32
	x   [sm3BlockSize]byte
	nx  int
	len uint64
}

// newSM3 returns a new hash.Hash computing the SM3 checksum.
func newSM3() hash.Hash {
	d := new(sm3Digest)
	d.Reset()
	return d
}

// Reset resets the hash to its initial state.
func (d *sm3Digest) Reset() {
	d.h = sm3IV
	d.nx = 0
	d.len = 0
}

// Size returns the SM3 digest size in bytes (32).
func (d *sm3Digest) Size() int { return sm3DigestSize }

// BlockSize returns the SM3 block size in bytes (64).
func (d *sm3Digest) BlockSize() int { return sm3BlockSize }

// Write adds data to the running hash.
func (d *sm3Digest) Write(p []byte) (int, error) {
	nn := len(p)
	d.len += uint64(nn)
	if d.nx > 0 {
		n := copy(d.x[d.nx:], p)
		d.nx += n
		if d.nx == sm3BlockSize {
			sm3Block(d, d.x[:])
			d.nx = 0
		}
		p = p[n:]
	}
	for len(p) >= sm3BlockSize {
		sm3Block(d, p[:sm3BlockSize])
		p = p[sm3BlockSize:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return nn, nil
}

// Sum appends the current hash to in and returns the result.
func (d *sm3Digest) Sum(in []byte) []byte {
	d0 := *d
	digest := d0.finalize()
	return append(in, digest[:]...)
}

// finalize pads the message and returns the final SM3 digest.
func (d *sm3Digest) finalize() [sm3DigestSize]byte {
	totalBits := d.len << 3
	mod := d.len % sm3BlockSize

	var padding [sm3BlockSize + 8]byte
	padding[0] = 0x80
	var padLen uint64
	if mod < 56 {
		padLen = 56 - mod
	} else {
		padLen = sm3BlockSize + 56 - mod
	}
	binary.BigEndian.PutUint64(padding[padLen:], totalBits)
	d.Write(padding[:padLen+8])

	var digest [sm3DigestSize]byte
	for i := range 8 {
		binary.BigEndian.PutUint32(digest[i*4:], d.h[i])
	}
	return digest
}

// sm3Block processes a single 64-byte block.
func sm3Block(d *sm3Digest, data []byte) {
	w, wp := sm3Expand(data)
	sm3Compress(d, &w, &wp)
}

// sm3Expand performs the SM3 message expansion.
func sm3Expand(data []byte) ([68]uint32, [64]uint32) {
	var w [68]uint32
	var wp [64]uint32
	for i := range 16 {
		w[i] = binary.BigEndian.Uint32(data[i*4:])
	}
	for i := 16; i < 68; i++ {
		w[i] = sm3P1(w[i-16]^w[i-9]^bits.RotateLeft32(w[i-3], 15)) ^
			bits.RotateLeft32(w[i-13], 7) ^ w[i-6]
	}
	for i := range 64 {
		wp[i] = w[i] ^ w[i+4]
	}
	return w, wp
}

// sm3Compress performs the SM3 compression function over 64 rounds.
func sm3Compress(d *sm3Digest, w *[68]uint32, wp *[64]uint32) {
	a, b, c, dd := d.h[0], d.h[1], d.h[2], d.h[3]
	e, f, g, hh := d.h[4], d.h[5], d.h[6], d.h[7]

	for j := range 16 {
		ss1, ss2 := sm3SS(a, e, 0x79cc4519, j)
		tt1 := (a ^ b ^ c) + dd + ss2 + wp[j]
		tt2 := (e ^ f ^ g) + hh + ss1 + w[j]
		a, b, c, dd, e, f, g, hh = sm3Shift(tt1, a, b, c, tt2, e, f, g)
	}
	for j := 16; j < 64; j++ {
		ss1, ss2 := sm3SS(a, e, 0x7a879d8a, j)
		tt1 := ((a & b) | (a & c) | (b & c)) + dd + ss2 + wp[j]
		tt2 := ((e & f) | (^e & g)) + hh + ss1 + w[j]
		a, b, c, dd, e, f, g, hh = sm3Shift(tt1, a, b, c, tt2, e, f, g)
	}

	d.h[0] ^= a
	d.h[1] ^= b
	d.h[2] ^= c
	d.h[3] ^= dd
	d.h[4] ^= e
	d.h[5] ^= f
	d.h[6] ^= g
	d.h[7] ^= hh
}

// sm3SS computes SS1 and SS2 for round j.
func sm3SS(a, e, t uint32, j int) (uint32, uint32) {
	ss1 := bits.RotateLeft32(
		bits.RotateLeft32(a, 12)+e+bits.RotateLeft32(t, j), 7,
	)
	ss2 := ss1 ^ bits.RotateLeft32(a, 12)
	return ss1, ss2
}

// sm3Shift performs the state register update for one round.
func sm3Shift(tt1, a, b, c, tt2, e, f, g uint32) (
	uint32, uint32, uint32, uint32, uint32, uint32, uint32, uint32,
) {
	return tt1, a, bits.RotateLeft32(b, 9), c,
		sm3P0(tt2), e, bits.RotateLeft32(f, 19), g
}

// sm3P0 is the SM3 permutation function P0.
func sm3P0(x uint32) uint32 {
	return x ^ bits.RotateLeft32(x, 9) ^ bits.RotateLeft32(x, 17)
}

// sm3P1 is the SM3 permutation function P1.
func sm3P1(x uint32) uint32 {
	return x ^ bits.RotateLeft32(x, 15) ^ bits.RotateLeft32(x, 23)
}
