package main

import (
	"KubeLite/data"
	"KubeLite/orchestrator"
)

func main() {
	data.InitRedis()
	go orchestrator.Orchestrate() //one thread that goes to call the orchestrator
}
