package main

import (
	"runtime"
	"sync"
	"time"
)

// StressCPUAllCores pegs ALL available CPU cores to ~100% for the given duration.
// It does this by spawning one goroutine per logical CPU, each running a tight
// floating-point math loop with no sleep or yield — forcing the OS scheduler
// to keep every core fully occupied.
//
// CPU Level  : HIGH (all cores simultaneously)
// Use case   : testing autoscaler scale-up trigger when CPU threshold is breached
//
// Example:
//
//	StressCPUAllCores(30) → burns all cores for 30 seconds, then stops cleanly
func StressCPUAllCores(durationSeconds int) {
	numCores := runtime.NumCPU()         // how many logical CPUs this machine has
	runtime.GOMAXPROCS(numCores)         // allow Go to use all of them

	done := make(chan struct{})           // shared signal to stop all goroutines
	var wg sync.WaitGroup

	// Spawn one goroutine per core — each runs a tight math loop
	for i := 0; i < numCores; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			burnCore(done)
		}()
	}

	// Let it burn for the requested duration, then signal all goroutines to stop
	time.Sleep(time.Duration(durationSeconds) * time.Second)
	close(done) // broadcast stop to every goroutine at once
	wg.Wait()   // wait for all goroutines to cleanly exit
}

// burnCore runs a tight floating-point math loop on a single core until
// the done channel is closed. No sleep, no yield — pure CPU pressure.
func burnCore(done <-chan struct{}) {
	x := 1.0000001
	for {
		select {
		case <-done:
			return // stop signal received, exit cleanly
		default:
			// Repeated multiply + divide in a hot loop
			// These ops are cheap individually but continuous and non-blocking,
			// which is exactly what forces the CPU to stay at 100% on this core
			for i := 0; i < 100_000; i++ {
				x *= 1.0000001
				x /= 1.0000001
			}
		}
	}
}
