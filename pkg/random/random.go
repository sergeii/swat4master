package random

import (
	crand "crypto/rand"
	mrand "math/rand/v2"
)

func RandInt(minVal, maxVal int) int {
	return mrand.IntN(maxVal-minVal) + minVal //nolint: gosec // no need for crypto/rand here
}

func RandBytes(sz int) []byte {
	data := make([]byte, sz)
	if _, err := crand.Read(data); err != nil {
		panic(err)
	}
	return data
}
