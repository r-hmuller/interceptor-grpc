package config

import (
	"net/http"
	"sync"
	"sync/atomic"
)

var processedMap sync.Map //1) Pending 2) Processed 3)snapshoted

var requestsMap sync.Map
var requestNumber atomic.Uint64

func SaveRequestToBuffer(request *http.Request) uint64 {
	num := requestNumber.Add(1)
	requestsMap.Store(num, request)
	processedMap.Store(num, "pending")
	return num
}

func UpdateRequestToProcessed(number uint64) {
	processedMap.Store(number, "processed")
}
