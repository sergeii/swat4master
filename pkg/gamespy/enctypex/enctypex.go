package enctypex

import "math/rand"

/*
The code has been borrowed and adapted from a work of Luigi Auriemma <aluigi@autistici.org>
http://aluigi.altervista.org/papers.htm
http://aluigi.altervista.org/papers/enctypex_decoder.c

The original license is included
*/

/*
Copyright 2008-2012 Luigi Auriemma

This program is free software; you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation; either version 2 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 59 Temple Place, Suite 330, Boston, MA  02111-1307 USA

http://www.gnu.org/licenses/gpl-2.0.txt
*/

const KL = 6   // game key length
const VL = 8   // validate key length
const HL = 23  // header length
const EL = 261 // encxkey length

func Encrypt(key [KL]byte, validate [VL]byte, data []byte) []byte {
	header := header(key, validate)
	hl := len(header)
	unprepared := make([]byte, hl+len(data))
	copy(unprepared, header[:])
	copy(unprepared[hl:], data)
	encxkey, prepared := prepare(key, validate, unprepared)
	encxdata := func6e(encxkey, prepared)
	encrypted := make([]byte, hl+len(encxdata))
	copy(encrypted, header[:])
	copy(encrypted[hl:], encxdata)
	return encrypted
}

func Decrypt(key [KL]byte, validate [VL]byte, encrypted []byte) []byte {
	encxkey, prepared := prepare(key, validate, encrypted)
	return func6(encxkey, prepared)
}

func prepare(key [KL]byte, validate [VL]byte, data []byte) ([EL]byte, []byte) {
	offset := (data[0] ^ 0xec) + 2
	count := data[offset-1] ^ 0xea
	encxkey := funcx(key, validate, data[offset:offset+count])
	return encxkey, data[offset+count:]
}

func header(key [KL]byte, validate [VL]byte) [HL]byte {
	var hdr [HL]byte
	rnd := rand.Int31() // nolint: gosec
	for i := range hdr {
		hdr[i] = uint8(rnd) ^ key[i%KL] ^ validate[i%VL]
	}
	hdr[0] = 0xeb
	hdr[1] = 0x00
	hdr[2] = 0x00
	hdr[8] = 0xe4
	return hdr
}

func funcx(key [KL]byte, validate [VL]byte, header []byte) [EL]byte {
	var encxvalidate [VL]byte
	copy(encxvalidate[:], validate[:])
	for i, _byte := range header {
		k := (key[i%KL] * uint8(i)) & 7
		encxvalidate[k] ^= encxvalidate[i&7] ^ _byte
	}
	return func4(encxvalidate)
}

func func4(encxvalidate [VL]byte) [EL]byte {
	var t1, n1, n2 int
	var encxkey [EL]byte
	for i := 0; i <= 255; i++ {
		encxkey[i] = uint8(i)
	}
	for i := 255; i >= 0; i-- {
		t1, n1, n2 = func5(encxkey, i, encxvalidate, n1, n2)
		encxkey[i], encxkey[t1] = encxkey[t1], encxkey[i]
	}
	encxkey[256] = encxkey[1]
	encxkey[257] = encxkey[3]
	encxkey[258] = encxkey[5]
	encxkey[259] = encxkey[7]
	encxkey[260] = encxkey[n1&0xFF]
	return encxkey
}

func func5(encxkey [EL]byte, cnt int, encxvalidate [VL]byte, n1 int, n2 int) (int, int, int) {
	var i, tmp int
	if cnt == 0 {
		return 0, n1, n2
	}
	mask := 1
	if cnt > 1 {
		for mask < cnt {
			mask = (mask << 1) + 1
		}
	}
	for {
		n1 = int(encxkey[n1&0xFF] + encxvalidate[n2])
		n2++
		if n2 >= VL {
			n2 = 0
			n1 += VL
		}
		tmp = n1 & mask
		i++
		if i > 11 {
			tmp %= cnt
		}
		if tmp <= cnt {
			break
		}
	}
	return tmp, n1, n2
}

func func6(encxkey [EL]byte, data []byte) []byte {
	for i := range data {
		data[i] = func7(encxkey[:], data[i])
	}
	return data
}

func func6e(encxkey [EL]byte, data []byte) []byte {
	for i := range data {
		data[i] = func7e(encxkey[:], data[i])
	}
	return data
}

func func7(encxkey []byte, d byte) byte {
	var a, b, c byte
	a = encxkey[256]
	b = encxkey[257]
	c = encxkey[a]
	encxkey[256] = a + 1
	encxkey[257] = b + c
	a = encxkey[260]
	b = encxkey[257]
	b = encxkey[b]
	c = encxkey[a]
	encxkey[a] = b
	a = encxkey[259]
	b = encxkey[257]
	a = encxkey[a]
	encxkey[b] = a
	a = encxkey[256]
	b = encxkey[259]
	a = encxkey[a]
	encxkey[b] = a
	a = encxkey[256]
	encxkey[a] = c
	b = encxkey[258]
	a = encxkey[c]
	c = encxkey[259]
	b += a
	encxkey[258] = b
	a = b
	c = encxkey[c]
	b = encxkey[257]
	b = encxkey[b]
	a = encxkey[a]
	c += b
	b = encxkey[260]
	b = encxkey[b]
	c += b
	b = encxkey[c]
	c = encxkey[256]
	c = encxkey[c]
	a += c
	c = encxkey[b]
	b = encxkey[a]
	encxkey[260] = d
	c ^= (b ^ d)
	encxkey[259] = c
	return c
}

func func7e(encxkey []byte, d byte) byte {
	var a, b, c byte
	a = encxkey[256]
	b = encxkey[257]
	c = encxkey[a]
	encxkey[256] = a + 1
	encxkey[257] = b + c
	a = encxkey[260]
	b = encxkey[257]
	b = encxkey[b]
	c = encxkey[a]
	encxkey[a] = b
	a = encxkey[259]
	b = encxkey[257]
	a = encxkey[a]
	encxkey[b] = a
	a = encxkey[256]
	b = encxkey[259]
	a = encxkey[a]
	encxkey[b] = a
	a = encxkey[256]
	encxkey[a] = c
	b = encxkey[258]
	a = encxkey[c]
	c = encxkey[259]
	b += a
	encxkey[258] = b
	a = b
	c = encxkey[c]
	b = encxkey[257]
	b = encxkey[b]
	a = encxkey[a]
	c += b
	b = encxkey[260]
	b = encxkey[b]
	c += b
	b = encxkey[c]
	c = encxkey[256]
	c = encxkey[c]
	a += c
	c = encxkey[b]
	b = encxkey[a]
	c ^= b ^ d
	encxkey[260] = c
	encxkey[259] = d
	return c
}
