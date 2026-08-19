package data

import (
	"sync/atomic"
)

var TotalRequest uint64

func IncrementRequestCount() { //first letter capital makes this func public
	atomic.AddUint64(&TotalRequest, 1) //multi-thread safe
}
func getRequestCount() uint64 {
	return TotalRequest
}
