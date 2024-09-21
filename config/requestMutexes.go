package config

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	pending = iota
	processed
	snapshoted
)

var processedMap sync.Map

var requestsMap sync.Map
var requestNumber atomic.Uint64

func GetLatestRequestNumber() uint64 {
	return requestNumber.Load()
}

func SaveRequestToBuffer(request *http.Request) uint64 {
	num := requestNumber.Add(1)
	requestsMap.Store(num, request)
	processedMap.Store(num, pending)
	return num
}

func UpdateRequestToProcessed(number uint64) {
	processedMap.Store(number, processed)
}

func UpdateRequestsToSnapshoted(latestRequest uint64) {
	processedMap.Range(func(key, value interface{}) bool {
		if key.(uint64) < latestRequest {
			processedMap.Store(key, snapshoted)
		}
		return true
	})

}

func ClearRequestsMap() {
	tick := time.Tick(60 * time.Second)
	for range tick {
		processedMap.Range(func(key, value interface{}) bool {
			if value == snapshoted {
				processedMap.Delete(key)
				requestsMap.Delete(key)
			}
			return true
		})
	}
}

func GetReprocessableRequests() []*http.Request {
	var reprocessableRequests []*http.Request
	processedMap.Range(func(key, value interface{}) bool {
		if value == pending || value == processed {
			request, _ := requestsMap.Load(key)
			reprocessableRequests = append(reprocessableRequests, request.(*http.Request))
		}
		return true
	})
	return reprocessableRequests
}
