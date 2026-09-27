package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

func main() {
	fmt.Println("Worker Node Starting...")

	rdb = InitRedisClient() // the returned client pointer is stored in the rdb variable
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown: listen for SIGTERM (sent by Master via ContainerStop).
	var isShuttingDown atomic.Bool
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, os.Interrupt)
	go func() {
		<-sigCh
		fmt.Println("Worker: SIGTERM received — finishing current task then exiting cleanly...")
		isShuttingDown.Store(true)
		cancel() // unblocks the BLPop call
	}()

	// Verify Redis connectivity
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		fmt.Printf(" Worker failed to connect to Redis at %s: %v\n", rdb.Options().Addr, err)
	} else {
		fmt.Printf("Worker successfully connected to Redis at %s: %s\n", rdb.Options().Addr, pong)
	}

	fmt.Println("Worker is now waiting for tasks on 'task_queue'...")
	
	// Start the infinite polling loop (located in worker.go)
	StartPolling(ctx, &isShuttingDown)
}
