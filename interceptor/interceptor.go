package interceptor

import (
	"bytes"
	"crypto/tls"
	"errors"
	"github.com/rs/zerolog/log"
	"interceptor-grpc/config"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

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

var clientLock = &sync.Mutex{}
var singleInstance *http.Client

func Handler(w http.ResponseWriter, r *http.Request) {
	requestNumber := config.SaveRequestToBuffer(r)

	r.URL.Host = config.GetApplicationURL()
	serverResponse := HTTPResponse{}
	serverResponse = sendRequest(r, requestNumber)

	w.WriteHeader(serverResponse.StatusCode)
	_, err := w.Write(serverResponse.Body)
	if err != nil {
		log.Err(err).Msg("Error writing response")
	}

	requestUpdate := config.RequestUpdate{
		Number: requestNumber,
		Status: "processed",
	}
	config.RequestUpdateChannel <- requestUpdate

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
		clientLock.Lock()
		if singleInstance == nil {
			singleInstance = &http.Client{Transport: tr}
		}
		clientLock.Unlock()
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
