package crypt

type cipherState struct {
	cards      [256]byte // A permutation of 0-255.
	rotor      byte      // Index that rotates smoothly
	ratchet    byte      // Index that moves erratically
	avalanche  byte      // Index heavily data dependent
	lastPlain  byte      // Last plain text byte
	lastCipher byte      // Last cipher text byte
}

func newCipherState(cryptKey [CRTL]byte) cipherState {
	var toswap, rsum, keypos uint8
	cs := cipherState{}
	// Start with state->cards all in order, one of each.
	for i := 0; i < 256; i++ {
		cs.cards[i] = uint8(i)
	}
	keypos = 0 // Start with first byte of the crypt key
	// Swap the card at each position with some other card.
	for i := 255; i >= 0; i-- {
		toswap, rsum, keypos = cs.shuffle(cryptKey, uint8(i), rsum, keypos)
		cs.cards[i], cs.cards[toswap] = cs.cards[toswap], cs.cards[i]
	}
	// Initialize the indices and data dependencies
	// Indices are set to different values instead of all 0
	// to reduce what is known about the state of the state->cards
	// when the first byte is emitted
	cs.rotor = cs.cards[1]
	cs.ratchet = cs.cards[3]
	cs.avalanche = cs.cards[5]
	cs.lastPlain = cs.cards[7]
	cs.lastCipher = cs.cards[rsum]
	return cs
}

func (cs *cipherState) shuffle(cryptKey [CRTL]byte, limit, rsum, keypos uint8) (uint8, uint8, uint8) {
	var u, mask uint8
	// Avoid divide by zero error
	if limit == 0 {
		return 0, rsum, keypos
	}
	mask = 1 // Fill mask with enough bits to cover
	for mask < limit {
		mask = (mask << 1) + 1
	}
	retries := 0 // To prevent infinite loops
	for {
		rsum = cs.cards[rsum] + cryptKey[keypos]
		keypos++
		if keypos >= CRTL {
			keypos = 0
			rsum += CRTL
		}
		u = mask & rsum
		retries++
		if retries > 11 {
			u %= limit // Prevent very rare long loops.
		}
		if u <= limit {
			break
		}
	}
	return u, rsum, keypos
}

func (cs *cipherState) Encrypt(data []byte) []byte {
	for i := 0; i < len(data); i++ {
		data[i] = cs.encryptByte(data[i])
	}
	return data
}

func (cs *cipherState) encryptByte(b byte) byte { // nolint: dupl
	// Picture a single enigma state->rotor with 256 positions, rewired
	// on the fly by card-shuffling.

	// This cipher is a variant of one invented and written
	// by Michael Paul Johnson in November 1993.
	var swaptemp byte

	// Shuffle the deck a little more.
	cs.ratchet += cs.cards[cs.rotor]
	cs.rotor++
	swaptemp = cs.cards[cs.lastCipher]
	cs.cards[cs.lastCipher] = cs.cards[cs.ratchet]
	cs.cards[cs.ratchet] = cs.cards[cs.lastPlain]
	cs.cards[cs.lastPlain] = cs.cards[cs.rotor]
	cs.cards[cs.rotor] = swaptemp
	cs.avalanche += cs.cards[swaptemp]

	// Output one byte from the state in such a way as to make it
	// very hard to figure out which one you are looking at.
	c := cs.cards[(cs.cards[cs.avalanche]+cs.cards[cs.rotor])&0xFF]
	d := cs.cards[cs.cards[(cs.cards[cs.lastPlain]+cs.cards[cs.lastCipher]+cs.cards[cs.ratchet])&0xFF]]
	cs.lastCipher = b ^ c ^ d
	cs.lastPlain = b
	return cs.lastCipher
}

func (cs *cipherState) Decrypt(data []byte) []byte {
	for i := 0; i < len(data); i++ {
		data[i] = cs.decryptByte(data[i])
	}
	return data
}

func (cs *cipherState) decryptByte(b byte) byte { // nolint: dupl
	var swaptemp byte
	// Shuffle the deck a little more
	cs.ratchet += cs.cards[cs.rotor]
	cs.rotor++
	swaptemp = cs.cards[cs.lastCipher]
	cs.cards[cs.lastCipher] = cs.cards[cs.ratchet]
	cs.cards[cs.ratchet] = cs.cards[cs.lastPlain]
	cs.cards[cs.lastPlain] = cs.cards[cs.rotor]
	cs.cards[cs.rotor] = swaptemp
	cs.avalanche += cs.cards[swaptemp]
	// crt - change this around
	c := cs.cards[(cs.cards[cs.avalanche]+cs.cards[cs.rotor])&0xFF]
	d := cs.cards[cs.cards[(cs.cards[cs.lastPlain]+cs.cards[cs.lastCipher]+cs.cards[cs.ratchet])&0xFF]]
	cs.lastPlain = b ^ c ^ d
	cs.lastCipher = b
	return cs.lastPlain
}
