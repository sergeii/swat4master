package filter

func eq[T comparable](this, that T) bool {
	return this == that
}

func ne[T comparable](this, that T) bool {
	return this != that
}

func lt(this, that int) bool {
	return this < that
}

func gt(this, that int) bool {
	return this > that
}
