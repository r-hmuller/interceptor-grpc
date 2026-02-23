package snapshotter

import (
	"context"
	"interceptor-grpc/config"
	"interceptor-grpc/crController"
	"interceptor-grpc/protos"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func GenerateSnapshots(ctx context.Context) {
	tick := time.Tick(time.Duration(config.GetCheckpointInterval()) * time.Second)
	for range tick {
		log.Info().Msg("Checkpoint interval reached, generating snapshot...")
		if config.IsSnapshotBeingTaken {
			log.Info().Msg("Snapshot is already being taken, skipping this interval.")
			continue
		}
		config.SnapshotLock.Lock()
		config.IsSnapshotBeingTaken = true
		config.SnapshotLock.Unlock()

		generateSnapshot(ctx)
	}
}

func generateSnapshot(ctx context.Context) {
	for crController.IsRunningPendingRequestQueue.Load() {
		time.Sleep(100 * time.Millisecond)
	}

	snapshotRequest := &protos.CreateSnapshotRequest{
		ServiceName:   config.GetServiceName(),
		RegistryName:  config.GetRegistryName(),
		Namespace:     config.GetNamespace(),
		LatestRequest: config.GetLatestRequestNumber(),
	}

	log.Info().Msg("Connecting to gRPC server at " + config.GetDaemonGrpcUrl())

	conn, err := grpc.NewClient(config.GetDaemonGrpcUrl(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Err(err).Msg("failed to connect to gRPC server at localhost:50051")
	}
	defer conn.Close()

	log.Info().Msg("Sending snapshot creation request to gRPC server...")
	log.Info().Msgf("Snapshot request details: ServiceName=%s, RegistryName=%s, Namespace=%s, LatestRequest=%d",
		snapshotRequest.ServiceName,
		snapshotRequest.RegistryName,
		snapshotRequest.Namespace,
		snapshotRequest.LatestRequest,
	)
	c := protos.NewSnapshotRPCServiceClient(conn)
	response, err := c.Create(ctx, snapshotRequest)
	if err != nil {
		log.Err(err).Msg("failed to create snapshot")
	}
	if response.GetResponse() != true {
		log.Error().Msg("failed to create snapshot: " + response.GetError())
	}
	config.IsSnapshotBeingTaken = false
}
