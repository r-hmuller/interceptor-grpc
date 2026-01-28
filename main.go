package main

import (
	"context"
	"crypto/tls"
	"github.com/gorilla/mux"
	"interceptor-grpc/config"
	"interceptor-grpc/crController"
	"interceptor-grpc/heartbeat"
	"interceptor-grpc/interceptor"
	"interceptor-grpc/snapshotter"
	"log"
	"net/http"
	"sync"
)

var ctx = context.Background()

func main() {
	config.VerifyEnvVars()

	var wg sync.WaitGroup
	wg.Add(1)
	go startListener()
	wg.Add(1)
	go interceptor.ProcessQueue()
	wg.Add(1)
	go crController.RunGRPCServer()
	wg.Add(1)
	go config.ClearRequestsMap()

	if config.GetHeartBeatEnabled() {
		log.Println("Heartbeat monitoring is enabled.")
		wg.Add(1)
		go heartbeat.Monitor()
	}
	if config.GetCheckpointEnabled() {
		log.Println("Checkpointing is enabled.")
		wg.Add(1)
		go snapshotter.GenerateSnapshots(ctx)
	} else {
		log.Println("Checkpointing is disabled.")
	}

	wg.Wait()
}

func startListener() {
	// Disable SSL validation, because some client may have invalid certificates
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	router := mux.NewRouter()
	router.PathPrefix("/_internal/pod/restart/start").HandlerFunc(crController.PodBeganRestarting)
	router.PathPrefix("/_internal/pod/restart/end").HandlerFunc(crController.PodEndedRestarting)
	router.PathPrefix("/").HandlerFunc(interceptor.Handler)

	log.Printf("Starting interceptor on port %s\n", config.GetInterceptorPort())
	log.Fatal(http.ListenAndServe(config.GetInterceptorPort(), router))
}
