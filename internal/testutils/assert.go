package testutils

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
