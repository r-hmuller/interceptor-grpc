package config

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
)

type RequestUpdate struct {
	Number uint64
	Status string
}

var processedMap sync.Map //1) Pending 2) Processed 3)snapshoted

var requestsMap sync.Map
var requestNumber atomic.Uint64

var RequestUpdateChannel = make(chan RequestUpdate)

func SaveRequestToBuffer(request *http.Request) uint64 {
	num := requestNumber.Add(1)
	requestsMap.Store(num, request)
	processedMap.Store(num, "pending")
	return num
}

func UpdateRequestToProcessed(number uint64) {
	processedMap.Store(number, "processed")
}

func SetRequestToNewStatus(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil

		case update := <-RequestUpdateChannel:
			processedMap.Store(update.Number, update.Status)

		}
	}
}
