package interceptor

import (
	"errors"
	"sync/atomic"
)

var QueueLength = atomic.Uint32{}
var queue = make([]QueueHttpRequest, 0)

func AddRequestToQueue(queueRequest QueueHttpRequest) {
	queue = append(queue, queueRequest)
	QueueLength.Add(1)
}

func GetRequestFromQueue() (QueueHttpRequest, error) {
	if len(queue) == 0 {
		return QueueHttpRequest{}, errors.New("queue is empty")
	}
	request := queue[0]
	queue = queue[1:]
	size := QueueLength.Load() - 1
	QueueLength.Store(size)
	return request, nil
}
