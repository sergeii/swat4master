package testutils

import (
	"fmt"
)

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func MustNoErr(err error) {
	if err != nil {
		panic(err)
	}
}

func Ignore(err error) {
	if err != nil {
		fmt.Printf("Error ignored: %v\n", err) // nolint:forbidigo
	}
}
