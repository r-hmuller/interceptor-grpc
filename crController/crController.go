package crController

import (
	"context"
	"google.golang.org/grpc"
	"interceptor-grpc/config"
	"interceptor-grpc/protos"
	"log"
	"net"
	"net/http"
	"sync/atomic"
)

var IsRunningPendingRequestQueue atomic.Bool
var IsDoingSnapshot atomic.Bool
var IsRestoringSnapshot atomic.Bool
var IsContainerUnavailable atomic.Bool

type server struct {
	protos.UnimplementedFailureServiceServer
	protos.UnimplementedSnapshotRPCServiceServer
}

func (s *server) StopRequests(_ context.Context, _ *protos.RestoreRequest) (*protos.RestoreResponse, error) {
	IsContainerUnavailable.Store(true)
	return &protos.RestoreResponse{Message: true}, nil
}

func (s *server) ReprocessRequests(_ context.Context, _ *protos.RestoreRequest) (*protos.RestoreResponse, error) {
	IsContainerUnavailable.Store(false)

	return &protos.RestoreResponse{Message: true}, nil
}

func (s *server) Reply(_ context.Context, replySnapshot *protos.ReplySnapshotRequest) (*protos.AckResponse, error) {
	// Aqui o snapshot já terminou, então precisa liberar os locks
	// Deletar todos os requests menores que o requestNumber
	// Prosseguir com os requests que estão na fila

	config.UpdateRequestsToSnapshoted(replySnapshot.LatestRequest)
	IsDoingSnapshot.Store(false)

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
		log.Fatalf("failed to listen on port %s: %v", config.GetSelfGrpcUrl(), err)
	}

	s := grpc.NewServer()
	protos.RegisterFailureServiceServer(s, &server{})
	log.Printf("gRPC server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
