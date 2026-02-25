package crController

import (
	"context"
	"net"
	"net/http"
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
	log.Info().Msg("StopRequests called - marking container as unavailable")
	IsContainerUnavailable.Store(true)
	IsRestoringSnapshot.Store(true)

	// Log current request stats
	pending, processed, snapshoted := config.GetRequestStats()
	log.Info().
		Int("pending", pending).
		Int("processed", processed).
		Int("snapshoted", snapshoted).
		Msg("Request stats when stopping")

	return &protos.RestoreResponse{Message: true}, nil
}

func (s *server) ReprocessRequests(_ context.Context, _ *protos.RestoreRequest) (*protos.RestoreResponse, error) {
	log.Info().Msg("ReprocessRequests called - starting recovery process")

	// Get all requests that need reprocessing
	reprocessableRequests := config.GetReprocessableRequests()

	log.Info().
		Int("count", len(reprocessableRequests)).
		Msg("Found requests to reprocess")

	// Reprocess each request
	reprocessedCount := 0
	failedCount := 0

	for _, bufferedReq := range reprocessableRequests {
		if bufferedReq.ResponseWriter == nil {
			log.Warn().
				Uint64("requestNumber", bufferedReq.RequestNumber).
				Msg("Cannot reprocess request: ResponseWriter is nil")
			failedCount++
			continue
		}

		// Check if connection is still alive by attempting to get the underlying connection
		// If the client has disconnected, the ResponseWriter won't work
		if reprocessCallback != nil {
			reprocessCallback(bufferedReq.Request, bufferedReq.ResponseWriter)
			config.MarkRequestForReprocessing(bufferedReq.RequestNumber)
			reprocessedCount++
			log.Debug().
				Uint64("requestNumber", bufferedReq.RequestNumber).
				Msg("Request added to reprocess queue")
		} else {
			log.Warn().Msg("Reprocess callback not registered - cannot reprocess requests")
			failedCount++
		}
	}

	log.Info().
		Int("reprocessed", reprocessedCount).
		Int("failed", failedCount).
		Msg("Recovery process completed")

	// Mark container as available again
	IsRestoringSnapshot.Store(false)
	IsContainerUnavailable.Store(false)

	return &protos.RestoreResponse{Message: true}, nil
}

func (s *server) Reply(_ context.Context, replySnapshot *protos.ReplySnapshotRequest) (*protos.AckResponse, error) {
	log.Info().
		Str("status", replySnapshot.SnapshotStatus).
		Uint64("latestRequest", replySnapshot.LatestRequest).
		Str("service", replySnapshot.ServiceName).
		Msg("Received snapshot reply from k8s-cr-daemon")

	// Mark all requests up to latestRequest as snapshoted
	config.UpdateRequestsToSnapshoted(replySnapshot.LatestRequest)

	// Log current stats after update
	pending, processed, snapshoted := config.GetRequestStats()
	log.Info().
		Int("pending", pending).
		Int("processed", processed).
		Int("snapshoted", snapshoted).
		Msg("Request stats after snapshot completion")

	// Release snapshot locks
	IsDoingSnapshot.Store(false)
	config.SnapshotLock.Lock()
	config.IsSnapshotBeingTaken = false
	config.SnapshotLock.Unlock()

	log.Info().Msg("Snapshot locks released, resuming normal operation")

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
	log.Info().Str("address", lis.Addr().String()).Msg("gRPC server listening")
	if err := s.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("Failed to serve gRPC")
	}
}
