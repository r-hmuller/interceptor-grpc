package interceptor

import (
	"bytes"
	"crypto/tls"
	"errors"
	"interceptor-grpc/config"
	"interceptor-grpc/crController"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var lock = &sync.RWMutex{}
var singleInstance *http.Client

type HTTPResponse struct {
	StatusCode         int
	Header             http.Header
	Body               []byte
	InterceptorControl string
}

type QueueHttpRequest struct {
	Request  *http.Request
	Response http.ResponseWriter
}

func ProcessQueue() {
	for {
		time.Sleep(50 * time.Millisecond)

		// Skip processing if container is unavailable, but don't exit the loop
		if crController.IsContainerUnavailable.Load() {
			crController.IsRunningPendingRequestQueue.Store(false)
			continue
		}

		// Skip if queue is empty, but keep the loop running
		if QueueLength.Load() == 0 {
			crController.IsRunningPendingRequestQueue.Store(false)
			continue
		}

		// Skip if snapshot or restore is in progress
		if crController.IsDoingSnapshot.Load() || crController.IsRestoringSnapshot.Load() {
			continue
		}

		request, err := GetRequestFromQueue()
		if err != nil {
			crController.IsRunningPendingRequestQueue.Store(false)
			continue
		}

		crController.IsRunningPendingRequestQueue.Store(true)

		// Process the request from the queue
		go processRequest(request.Response, request.Request)
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	timeout := 5 * time.Minute

	for crController.IsDoingSnapshot.Load() ||
		crController.IsRestoringSnapshot.Load() ||
		crController.IsContainerUnavailable.Load() {
		if time.Since(startTime) > timeout {
			http.Error(w, "request timed out while waiting for container to be available", http.StatusBadGateway)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	processRequest(w, r)
}

func processRequest(responseWriter http.ResponseWriter, request *http.Request) {
	// Check if it can be processed - if unavailable, queue and return
	if crController.IsUnavailable() {
		AddRequestToQueue(QueueHttpRequest{Request: request, Response: responseWriter})
		return
	}

	// Save request with ResponseWriter for potential reprocessing after recovery
	requestNumber := config.SaveRequestToBuffer(request, responseWriter)

	request.URL.Host = config.GetApplicationURL()
	serverResponse := HTTPResponse{}
	serverResponse = sendRequest(request, requestNumber)

	responseWriter.WriteHeader(serverResponse.StatusCode)
	_, err := responseWriter.Write(serverResponse.Body)
	if err != nil {
		log.Err(err).Msg("Error writing response")
	}

	config.UpdateRequestToProcessed(requestNumber)
}

func sendRequest(destiny *http.Request, uuid uint64) HTTPResponse {
	response := HTTPResponse{}
	client := getHttpClient()
	method := destiny.Method

	requestBody, err := io.ReadAll(destiny.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		response.StatusCode = 500
		return response
	}

	fullPath := config.GetApplicationURL() + destiny.URL.Path + "?" + destiny.URL.RawQuery

	req, err := http.NewRequest(method, fullPath, bytes.NewReader(requestBody))
	if err != nil {
		log.Err(err).Msg("Error creating request")
		response.StatusCode = 500
		return response
	}

	req.Header.Add("Interceptor-Controller", strconv.FormatUint(uuid, 10))
	addHeaders(destiny, req)
	resp, err := client.Do(req)
	if err != nil {
		log.Err(err).Msg("Error sending request")
		response.StatusCode = 500
		return response
	}
	response.StatusCode = resp.StatusCode
	response.Header = resp.Header
	body, err := getBodyContent(resp)
	if err != nil {
		log.Err(err).Msg("Error getting body content")
		response.StatusCode = 500
		return response
	}
	err = resp.Body.Close()
	if err != nil {
		log.Err(err).Msg("Error closing response body")
		response.StatusCode = 500
		return response
	}
	log.Info().Msgf("Body: %s", string(body))

	response.Body = body
	response.InterceptorControl = strconv.FormatUint(uuid, 10)

	return response
}

func getHttpClient() *http.Client {
	tr := &http.Transport{
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 500000,
		IdleConnTimeout:     5 * time.Second,
		DisableCompression:  true,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	if singleInstance == nil {
		lock.Lock()
		if singleInstance == nil {
			singleInstance = &http.Client{Transport: tr}
		}
		lock.Unlock()
	}

	return singleInstance
}

func getBodyContent(response *http.Response) ([]byte, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Err(err).Msg("Error reading response body")
		return nil, errors.New("error parsing request body")
	}
	return body, nil
}

func addHeaders(original *http.Request, created *http.Request) {
	for name, values := range original.Header {
		for _, value := range values {
			created.Header.Add(name, value)
		}
	}
}
