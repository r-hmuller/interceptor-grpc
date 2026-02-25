package config

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	Pending = iota
	Processed
	Snapshoted
)

// BufferedRequest stores both the request and response writer for potential reprocessing
type BufferedRequest struct {
	Request        *http.Request
	ResponseWriter http.ResponseWriter
	RequestNumber  uint64
	State          int
}

var processedMap sync.Map
var requestsMap sync.Map
var requestNumber atomic.Uint64
var requestsMapMutex sync.RWMutex

func GetLatestRequestNumber() uint64 {
	return requestNumber.Load()
}

// SaveRequestToBuffer stores the request with its ResponseWriter for potential reprocessing
func SaveRequestToBuffer(request *http.Request, responseWriter http.ResponseWriter) uint64 {
	num := requestNumber.Add(1)

	bufferedReq := &BufferedRequest{
		Request:        request,
		ResponseWriter: responseWriter,
		RequestNumber:  num,
		State:          Pending,
	}

	requestsMapMutex.Lock()
	requestsMap.Store(num, bufferedReq)
	requestsMapMutex.Unlock()

	processedMap.Store(num, Pending)
	return num
}

func UpdateRequestToProcessed(number uint64) {
	processedMap.Store(number, Processed)

	// Also update the buffered request state
	requestsMapMutex.Lock()
	if val, ok := requestsMap.Load(number); ok {
		if bufferedReq, ok := val.(*BufferedRequest); ok {
			bufferedReq.State = Processed
		}
	}
	requestsMapMutex.Unlock()
}

func UpdateRequestsToSnapshoted(latestRequest uint64) {
	processedMap.Range(func(key, value interface{}) bool {
		// Use <= to include the request with ID equal to latestRequest
		if key.(uint64) <= latestRequest {
			processedMap.Store(key, Snapshoted)

			// Also update the buffered request state
			requestsMapMutex.Lock()
			if val, ok := requestsMap.Load(key); ok {
				if bufferedReq, ok := val.(*BufferedRequest); ok {
					bufferedReq.State = Snapshoted
				}
			}
			requestsMapMutex.Unlock()
		}
		return true
	})
}

func ClearRequestsMap() {
	tick := time.Tick(60 * time.Second)
	for range tick {
		var keysToDelete []interface{}

		processedMap.Range(func(key, value interface{}) bool {
			if value == Snapshoted {
				keysToDelete = append(keysToDelete, key)
			}
			return true
		})

		requestsMapMutex.Lock()
		for _, key := range keysToDelete {
			processedMap.Delete(key)
			requestsMap.Delete(key)
		}
		requestsMapMutex.Unlock()

		if len(keysToDelete) > 0 {
			log.Debug().Int("count", len(keysToDelete)).Msg("Cleared snapshoted requests from memory")
		}
	}
}

// GetReprocessableRequests returns all requests that are pending or processed but not snapshoted
func GetReprocessableRequests() []*BufferedRequest {
	var reprocessableRequests []*BufferedRequest

	requestsMapMutex.RLock()
	defer requestsMapMutex.RUnlock()

	processedMap.Range(func(key, value interface{}) bool {
		state := value.(int)
		if state == Pending || state == Processed {
			if val, ok := requestsMap.Load(key); ok {
				if bufferedReq, ok := val.(*BufferedRequest); ok {
					reprocessableRequests = append(reprocessableRequests, bufferedReq)
				}
			}
		}
		return true
	})

	return reprocessableRequests
}

// GetRequestStats returns counts of requests in each state for monitoring
func GetRequestStats() (pending, processed, snapshoted int) {
	processedMap.Range(func(key, value interface{}) bool {
		switch value.(int) {
		case Pending:
			pending++
		case Processed:
			processed++
		case Snapshoted:
			snapshoted++
		}
		return true
	})
	return
}

// MarkRequestForReprocessing resets a processed request back to pending state
func MarkRequestForReprocessing(requestNum uint64) {
	processedMap.Store(requestNum, Pending)

	requestsMapMutex.Lock()
	if val, ok := requestsMap.Load(requestNum); ok {
		if bufferedReq, ok := val.(*BufferedRequest); ok {
			bufferedReq.State = Pending
		}
	}
	requestsMapMutex.Unlock()
}
