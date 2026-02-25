package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"interceptor-grpc/config"
	"interceptor-grpc/crController"
	"interceptor-grpc/heartbeat"
	"interceptor-grpc/interceptor"
	"interceptor-grpc/snapshotter"
)

var ctx = context.Background()

func main() {
	config.VerifyEnvVars()

	// Register the reprocess callback for recovery mechanism
	crController.RegisterReprocessCallback(interceptor.AddToQueueForReprocess)

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
		log.Info().Msg("Heartbeat monitoring is enabled")
		wg.Add(1)
		go heartbeat.Monitor()
	}
	if config.GetCheckpointEnabled() {
		log.Info().Msg("Checkpointing is enabled")
		wg.Add(1)
		go snapshotter.GenerateSnapshots(ctx)
	} else {
		log.Info().Msg("Checkpointing is disabled")
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

	log.Info().Str("port", config.GetInterceptorPort()).Msg("Starting interceptor")
	if err := http.ListenAndServe(config.GetInterceptorPort(), router); err != nil {
		log.Fatal().Err(err).Msg("Failed to start HTTP server")
	}
}
