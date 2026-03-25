package crController

import (
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"interceptor-grpc/config"
	"interceptor-grpc/protos"
)

var IsRunningPendingRequestQueue atomic.Bool
var IsDoingSnapshot atomic.Bool
var IsRestoringSnapshot atomic.Bool
var IsContainerUnavailable atomic.Bool
var InFlightRequests sync.WaitGroup

// ReprocessCallback is a function type for adding requests back to the queue
// This callback is set by the interceptor package to avoid circular imports
type ReprocessCallback func(request *http.Request, responseWriter http.ResponseWriter)

var reprocessCallback ReprocessCallback

// RegisterReprocessCallback allows the interceptor package to register its AddRequestToQueue function
func RegisterReprocessCallback(callback ReprocessCallback) {
	reprocessCallback = callback
}

type server struct {
	protos.UnimplementedFailureServiceServer
	protos.UnimplementedSnapshotRPCServiceServer
}

func (s *server) StopRequests(_ context.Context, _ *protos.RestoreRequest) (*protos.RestoreResponse, error) {
	IsContainerUnavailable.Store(true)
	IsRestoringSnapshot.Store(true)
	return &protos.RestoreResponse{Message: true}, nil
}

func (s *server) ReprocessRequests(_ context.Context, _ *protos.RestoreRequest) (*protos.RestoreResponse, error) {
	reprocessableRequests := config.GetReprocessableRequests()

	for _, bufferedReq := range reprocessableRequests {
		if bufferedReq.ResponseWriter == nil {
			log.Warn().
				Uint64("requestNumber", bufferedReq.RequestNumber).
				Msg("Cannot reprocess request: ResponseWriter is nil")
			continue
		}

		if reprocessCallback != nil {
			reprocessCallback(bufferedReq.Request, bufferedReq.ResponseWriter)
			config.MarkRequestForReprocessing(bufferedReq.RequestNumber)
		} else {
			log.Warn().Msg("Reprocess callback not registered")
		}
	}

	IsRestoringSnapshot.Store(false)
	IsContainerUnavailable.Store(false)

	return &protos.RestoreResponse{Message: true}, nil
}

func (s *server) Reply(_ context.Context, replySnapshot *protos.ReplySnapshotRequest) (*protos.AckResponse, error) {
	log.Info().
		Str("status", replySnapshot.SnapshotStatus).
		Str("service", replySnapshot.ServiceName).
		Uint64("latestRequest", replySnapshot.LatestRequest).
		Msg("Snapshot Reply received from daemon")

	config.UpdateRequestsToSnapshoted(replySnapshot.LatestRequest)

	IsDoingSnapshot.Store(false)
	config.SnapshotLock.Lock()
	config.IsSnapshotBeingTaken = false
	config.SnapshotLock.Unlock()

	log.Info().Msg("Snapshot complete, requests unblocked")

	return &protos.AckResponse{Response: true, Error: ""}, nil
}

func IsUnavailable() bool {
	return IsRunningPendingRequestQueue.Load() || IsDoingSnapshot.Load() || IsRestoringSnapshot.Load() || IsContainerUnavailable.Load()
}

func PodBeganRestarting(w http.ResponseWriter, _ *http.Request) {
	IsContainerUnavailable.Store(true)
	w.WriteHeader(http.StatusNoContent)
}

func PodEndedRestarting(w http.ResponseWriter, _ *http.Request) {
	IsContainerUnavailable.Store(false)
	w.WriteHeader(http.StatusNoContent)
}

func RunGRPCServer() {
	lis, err := net.Listen("tcp", config.GetSelfGrpcUrl())
	if err != nil {
		log.Fatal().Err(err).Str("port", config.GetSelfGrpcUrl()).Msg("Failed to listen on port")
	}

	s := grpc.NewServer()
	protos.RegisterFailureServiceServer(s, &server{})
	protos.RegisterSnapshotRPCServiceServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("Failed to serve gRPC")
	}
}
