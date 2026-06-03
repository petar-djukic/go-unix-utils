// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/binary"
	"hash"
	"math/bits"
)

const sm3Size = 32
const sm3BlockSize = 64

var sm3IV = [8]uint32{
	0x7380166f, 0x4914b2b9, 0x172442d7, 0xda8a0600,
	0xa96f30bc, 0x163138aa, 0xe38dee4d, 0xb0fb0e4e,
}

type sm3digest struct {
	h   [8]uint32
	x   [sm3BlockSize]byte
	nx  int
	len uint64
}

func newSM3() hash.Hash {
	d := new(sm3digest)
	d.Reset()
	return d
}

func (d *sm3digest) Reset()         { d.h = sm3IV; d.nx = 0; d.len = 0 }
func (d *sm3digest) Size() int      { return sm3Size }
func (d *sm3digest) BlockSize() int { return sm3BlockSize }

func (d *sm3digest) Write(p []byte) (int, error) {
	nn := len(p)
	d.len += uint64(nn)
	if d.nx > 0 {
		n := copy(d.x[d.nx:], p)
		d.nx += n
		if d.nx == sm3BlockSize {
			d.block(d.x[:])
			d.nx = 0
		}
		p = p[n:]
	}
	for len(p) >= sm3BlockSize {
		d.block(p[:sm3BlockSize])
		p = p[sm3BlockSize:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return nn, nil
}

func (d *sm3digest) Sum(in []byte) []byte {
	d0 := *d
	digest := d0.checkSum()
	return append(in, digest[:]...)
}

func (d *sm3digest) checkSum() [sm3Size]byte {
	var tmp [sm3BlockSize + 8]byte
	tmp[0] = 0x80
	bitLen := d.len << 3
	padLen := 55 - int(d.len%sm3BlockSize)
	if padLen < 0 {
		padLen += sm3BlockSize
	}
	binary.BigEndian.PutUint64(tmp[1+padLen:], bitLen)
	d.Write(tmp[:1+padLen+8])
	var digest [sm3Size]byte
	for i := range 8 {
		binary.BigEndian.PutUint32(digest[i*4:], d.h[i])
	}
	return digest
}

func (d *sm3digest) block(data []byte) {
	var w [68]uint32
	for i := range 16 {
		w[i] = binary.BigEndian.Uint32(data[i*4:])
	}
	for j := 16; j < 68; j++ {
		x := w[j-16] ^ w[j-9] ^ bits.RotateLeft32(w[j-3], 15)
		w[j] = (x ^ bits.RotateLeft32(x, 15) ^ bits.RotateLeft32(x, 23)) ^
			bits.RotateLeft32(w[j-13], 7) ^ w[j-6]
	}
	v := d.h
	for j := range 64 {
		t := uint32(0x79cc4519)
		if j >= 16 {
			t = 0x7a879d8a
		}
		ss1 := bits.RotateLeft32(bits.RotateLeft32(v[0], 12)+v[4]+bits.RotateLeft32(t, j), 7)
		ss2 := ss1 ^ bits.RotateLeft32(v[0], 12)
		var tt1, tt2 uint32
		if j < 16 {
			tt1 = (v[0] ^ v[1] ^ v[2]) + v[3] + ss2 + (w[j] ^ w[j+4])
			tt2 = (v[4] ^ v[5] ^ v[6]) + v[7] + ss1 + w[j]
		} else {
			tt1 = ((v[0]&v[1])|(v[0]&v[2])|(v[1]&v[2])) + v[3] + ss2 + (w[j] ^ w[j+4])
			tt2 = ((v[4]&v[5])|(^v[4]&v[6])) + v[7] + ss1 + w[j]
		}
		v[3], v[2], v[1], v[0] = v[2], bits.RotateLeft32(v[1], 9), v[0], tt1
		v[7], v[6], v[5], v[4] = v[6], bits.RotateLeft32(v[5], 19), v[4],
			tt2^bits.RotateLeft32(tt2, 9)^bits.RotateLeft32(tt2, 17)
	}
	for i := range 8 {
		d.h[i] ^= v[i]
	}
}
