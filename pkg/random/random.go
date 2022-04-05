package random

import (
	crand "crypto/rand"
	"encoding/binary"
	mrand "math/rand"
)

func Seed() error {
	var buf [8]byte
	if _, err := crand.Read(buf[:]); err != nil {
		return err
	}
	mrand.Seed(int64(binary.LittleEndian.Uint64(buf[:])))
	return nil
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
