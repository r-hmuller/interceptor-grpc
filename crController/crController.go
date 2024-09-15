package crController

import (
	"context"
	"google.golang.org/grpc"
	crController "interceptor-grpc/protos"
	"log"
	"net"
	"sync/atomic"
)

var IsRunningPendingRequestQueue atomic.Bool
var IsDoingSnapshot atomic.Bool
var IsRestoringSnapshot atomic.Bool
var IsContainerUnavailable atomic.Bool

type server struct {
	crController.UnimplementedFailureServiceServer
}

func (s *server) StopRequests(_ context.Context, _ *crController.RestoreRequest) (*crController.RestoreResponse, error) {
	IsContainerUnavailable.Store(true)
	return &crController.RestoreResponse{Message: true}, nil
}

func (s *server) ReprocessRequests(_ context.Context, _ *crController.RestoreRequest) (*crController.RestoreResponse, error) {
	IsContainerUnavailable.Store(false)

	return &crController.RestoreResponse{Message: true}, nil
}

func IsUnavailable() bool {
	return IsRunningPendingRequestQueue.Load() || IsDoingSnapshot.Load() || IsRestoringSnapshot.Load() || IsContainerUnavailable.Load()
}

func RunGRPCServer() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen on port 50051: %v", err)
	}

	s := grpc.NewServer()
	crController.RegisterFailureServiceServer(s, &server{})
	log.Printf("gRPC server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
