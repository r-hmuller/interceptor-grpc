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
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"time"
)

func main() {
	duration := 160 * time.Second

	if config.GetEnableTrace() {
		go func() {
			f, _ := os.Create("trace.out")
			trace.Start(f)
			start := time.Now()

			for time.Since(start) < duration {
				// Your code here
				time.Sleep(1 * time.Second) // Adjust the sleep duration as needed
			}
			trace.Stop()
		}()

		go func() {
			f, err := os.Create("cpu.pprof")
			if err != nil {
				log.Fatal("could not create CPU profile: ", err)
			}

			if err := pprof.StartCPUProfile(f); err != nil {
				log.Fatal("could not start CPU profile: ", err)
			}
			start := time.Now()
			for time.Since(start) < duration {
				// Your code here
				time.Sleep(1 * time.Second) // Adjust the sleep duration as needed
			}
			pprof.StopCPUProfile()
			f.Close()
		}()

		go func() {
			f, err := os.Create("mem.pprof")
			if err != nil {
				log.Fatal("could not create memory profile: ", err)
			}

			start := time.Now()
			for time.Since(start) < duration {
				// Your code here
				time.Sleep(1 * time.Second) // Adjust the sleep duration as needed
			}
			pprof.WriteHeapProfile(f)
			f.Close()
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
