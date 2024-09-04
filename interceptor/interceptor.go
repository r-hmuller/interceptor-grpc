package interceptor

import (
	"bytes"
	"crypto/tls"
	"errors"
	"github.com/rs/zerolog/log"
	"interceptor-grpc/config"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"
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

func Handler(w http.ResponseWriter, r *http.Request) {
	processRequest(w, r)

}

func processRequest(responseWriter http.ResponseWriter, request *http.Request) {
	startTime := time.Now()
	requestNumber := config.SaveRequestToBuffer(request)

	request.URL.Host = config.GetApplicationURL()
	serverResponse := HTTPResponse{}
	serverResponse = sendRequest(request, requestNumber)

	responseWriter.WriteHeader(serverResponse.StatusCode)
	_, err := responseWriter.Write(serverResponse.Body)
	if err != nil {
		log.Err(err).Msg("Error writing response")
	}

	config.UpdateRequestToProcessed(requestNumber)
	elapsedTime := time.Since(startTime)
	log.Info().Msgf("All flow for request %d took %s", requestNumber, elapsedTime)
}

func sendRequest(destiny *http.Request, uuid uint64) HTTPResponse {
	startTime := time.Now()
	response := HTTPResponse{}
	client := getHttpClient()
	method := destiny.Method

	requestBody, err := io.ReadAll(destiny.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		response.StatusCode = 500
		return response
	}

	log.Info().Msgf("Path: %s", destiny.URL.Path)
	log.Info().Msgf("Query: %s", destiny.URL.RawQuery)
	log.Info().Msgf("Query: %s", destiny.URL.Query().Encode())
	fullPath := config.GetApplicationURL() + destiny.URL.Path + "?" + destiny.URL.RawQuery

	log.Info().Msgf("Sending request %d to %s", uuid, fullPath)

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

	log.Info().Msgf("Response for request %d: %d", uuid, response.StatusCode)
	elapsedTime := time.Since(startTime)
	log.Info().Msgf("Send request %d took %s", uuid, elapsedTime)
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
	body, err := ioutil.ReadAll(response.Body)
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
