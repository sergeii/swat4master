package random

import (
	crand "crypto/rand"
	mrand "math/rand"
)

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
