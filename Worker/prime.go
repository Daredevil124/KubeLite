package main

// FindNthPrime calculates the N-th prime number using CPU-heavy trial division.
// Brute Force method is chosen intentionally to purely stress test the CPU.

func FindNthPrime(n int) int{
	count := 0   //Keeping a count of number of prime numbers visited
	candidate := 1  // Number that is being tested
	for count < n {
		candidate++
		isPrime:=true
		for i := 2; i < candidate; i++ {
			if candidate % i == 0 {
				isPrime = false
				break
			}
		}
		if isPrime {
			count++
		}
	}
	return candidate;
}