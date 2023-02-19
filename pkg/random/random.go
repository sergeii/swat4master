package random

import (
	crand "crypto/rand"
	"encoding/binary"
	mrand "math/rand"
)

func Seed() (int64, error) {
	var buf [8]byte
	if _, err := crand.Read(buf[:]); err != nil {
		return -1, err
	}
	seed := int64(binary.LittleEndian.Uint64(buf[:]))
	mrand.Seed(seed)
	return seed, nil
}

func RandInt(min, max int) int {
	return mrand.Intn(max-min) + min // nolint: gosec
}

func RandBytes(sz int) []byte {
	data := make([]byte, sz)
	if _, err := crand.Read(data); err != nil {
		panic(err)
	}
	return data
}
