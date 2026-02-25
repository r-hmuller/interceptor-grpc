package interceptor

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
)

var QueueLength = atomic.Uint32{}
var queue = make([]QueueHttpRequest, 0)
var queueMutex sync.Mutex

func AddRequestToQueue(queueRequest QueueHttpRequest) {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	queue = append(queue, queueRequest)
	QueueLength.Add(1)

	log.Debug().
		Uint32("queueLength", QueueLength.Load()).
		Msg("Request added to queue")
}

// AddToQueueForReprocess is a callback-friendly wrapper for AddRequestToQueue
// Used by crController for recovery mechanism
func AddToQueueForReprocess(request *http.Request, responseWriter http.ResponseWriter) {
	AddRequestToQueue(QueueHttpRequest{
		Request:  request,
		Response: responseWriter,
	})
}

func GetRequestFromQueue() (QueueHttpRequest, error) {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	if len(queue) == 0 {
		return QueueHttpRequest{}, errors.New("queue is empty")
	}
	request := queue[0]
	queue = queue[1:]
	QueueLength.Store(uint32(len(queue)))
	return request, nil
}
