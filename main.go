package main

import (
	"crypto/tls"
	"github.com/gorilla/mux"
	"interceptor-grpc/config"
	"interceptor-grpc/interceptor"
	"log"
	"net/http"
	"sync"
)

func main() {
	config.VerifyEnvVars()

	var wg sync.WaitGroup
	wg.Add(1)
	go startListener()

	wg.Wait()
}

func startListener() {
	// Disable SSL validation, because some client may have invalid certificates
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	router := mux.NewRouter()
	router.PathPrefix("/").HandlerFunc(interceptor.Handler)

	log.Fatal(http.ListenAndServe(config.GetInterceptorPort(), router))
}
