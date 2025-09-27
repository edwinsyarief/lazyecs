package lazyecs

// max returns the larger of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// extendSlice extends a slice by n elements, reallocating if necessary.
func extendSlice[T any](s []T, n int) []T {
	newLen := len(s) + n
	if cap(s) >= newLen {
		return s[:newLen]
	}
	newCap := max(2*cap(s), newLen)
	ns := make([]T, newLen, newCap)
	copy(ns, s)
	return ns
}

// extendByteSlice extends a byte slice by n bytes, reallocating if necessary.
func extendByteSlice(s []byte, n int) []byte {
	newLen := len(s) + n
	if cap(s) >= newLen {
		return s[:newLen]
	}
	newCap := max(2*cap(s), newLen)
	ns := make([]byte, newLen, newCap)
	copy(ns, s)
	return ns
}
