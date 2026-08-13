package data

import (
	"sync/atomic"
)

var TotalRequest uint64

func incrementRequestCount() {
	atomic.AddUint64(&TotalRequest, 1)
}
func getRequestCount() uint64 {
	return TotalRequest
}
