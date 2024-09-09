package main

import (
	"context"
	"crypto/tls"
	"github.com/gorilla/mux"
	"interceptor-grpc/config"
	"interceptor-grpc/interceptor"
	"log"
	"net/http"
	"os"
	"runtime/trace"
	"sync"
	"time"
)

func main() {
	if config.GetEnableTrace() {
		go func() {
			f, _ := os.Create("trace.out")
			trace.Start(f)
			start := time.Now()
			duration := 153 * time.Second

			for time.Since(start) < duration {
				// Your code here
				time.Sleep(1 * time.Second) // Adjust the sleep duration as needed
			}
			trace.Stop()
		}()
	}

	config.VerifyEnvVars()

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)
	go startListener()
	wg.Add(1)
	go func() {
		err := config.SetRequestToNewStatus(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()
}

func startListener() {
	// Disable SSL validation, because some client may have invalid certificates
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	router := mux.NewRouter()
	router.PathPrefix("/").HandlerFunc(interceptor.Handler)

	log.Printf("Starting interceptor on port %s\n", config.GetInterceptorPort())
	log.Fatal(http.ListenAndServe(config.GetInterceptorPort(), router))
}
