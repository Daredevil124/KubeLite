package tasks

// This function returns true if n is a power of two.
func IsPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}
