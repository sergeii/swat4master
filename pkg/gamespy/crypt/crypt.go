package crypt

/*
This server browser encryption package is a derivative work
of the encryption algorithm borrowed from GameSpy SDK,
originally implemented in C, rewritten in Go and adapted for this project's needs.

The original license, as follows:

-------

Copyright (c) 2011, IGN Entertainment, Inc. ("IGN")
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

- Redistributions of source code must retain the above copyright notice, this
list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice,
this list of conditions and the following disclaimer in the documentation
and/or other materials provided with the distribution.
- Neither the name of IGN nor the names of its contributors may be used to
endorse or promote products derived from this software without specific
prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.
*/

import (
	"github.com/sergeii/swat4master/pkg/random"
)

const (
	GMSL = 6        // game secret length
	SCHL = 14       // server challenge length
	CCHL = 8        // client challenge length
	CRTL = CCHL     // crypt key length, same as client challenge
	HDRL = 9 + SCHL // header length
)

func Encrypt(gameSecret [GMSL]byte, challenge [CCHL]byte, data []byte) []byte {
	var cryptKey [CRTL]byte
	// prepare an encrypted payload that is then sent to a client
	// the first 23 bytes is the header, the rest is 1:1 ciphertext
	payload := make([]byte, HDRL+len(data))
	// init crypt header and fill it with random
	// bytes 9-23 will be the crypt key xor'ed with the game secret and the client's challenge
	for i := 0; i < HDRL; i++ {
		payload[i] = uint8(random.RandInt(1, 255)) ^ gameSecret[i%GMSL] ^ challenge[i%CCHL]
	}
	svrChallenge := payload[9:HDRL]
	copy(cryptKey[:], challenge[:])
	for i, b := range svrChallenge {
		cryptKey[(uint8(i)*gameSecret[i%GMSL])%CCHL] ^= (cryptKey[i%CCHL] ^ b) & 0xFF
	}
	payload[0] = 0xeb // ^0xec + 2 = 9 - offset of the crypt key in the resulting payload
	payload[1] = 0x00 // this and the next byte - query backend options, short int, always zero for swat
	payload[2] = 0x00
	payload[8] = SCHL ^ 0xea // ^ 0xea = 14 - crypt key length, i.e. bytes [9...23)
	state := newCipherState(cryptKey)
	ciphertext := (&state).Encrypt(data)
	copy(payload[HDRL:], ciphertext)
	return payload
}

func Decrypt(gameSecret [GMSL]byte, clientChallenge [CCHL]byte, data []byte) []byte {
	var i uint8
	var cryptKey [CRTL]byte
	// combine secret key, client and server challenges into a crypt key
	svrChOffset := (data[0] ^ 0xec) + 2                      // 9
	svrChLen := data[svrChOffset-1] ^ 0xea                   // 14
	svrChallenge := data[svrChOffset : svrChOffset+svrChLen] // [9..23)
	copy(cryptKey[:], clientChallenge[:])
	for i = 0; i < svrChLen; i++ {
		k := (i * gameSecret[i%GMSL]) % CCHL
		cryptKey[k] ^= (cryptKey[i%CCHL] ^ svrChallenge[i]) & 0xFF
	}
	// the encrypted data is the remaining payload
	ciphertext := data[svrChOffset+svrChLen:] // [23...]
	state := newCipherState(cryptKey)
	return (&state).Decrypt(ciphertext)
}
