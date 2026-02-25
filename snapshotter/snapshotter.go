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

		// Lock before checking to prevent race condition
		config.SnapshotLock.Lock()
		if config.IsSnapshotBeingTaken {
			config.SnapshotLock.Unlock()
			log.Info().Msg("Snapshot is already being taken, skipping this interval.")
			continue
		}
		config.IsSnapshotBeingTaken = true
		config.SnapshotLock.Unlock()

		generateSnapshot(ctx)
	}
}

// releaseSnapshotLocks releases all snapshot-related locks in case of failure
func releaseSnapshotLocks() {
	crController.IsDoingSnapshot.Store(false)
	config.SnapshotLock.Lock()
	config.IsSnapshotBeingTaken = false
	config.SnapshotLock.Unlock()
}

func generateSnapshot(ctx context.Context) {
	// Wait for pending queue to drain with timeout
	waitStart := time.Now()
	maxWaitTime := 30 * time.Second
	for crController.IsRunningPendingRequestQueue.Load() {
		if time.Since(waitStart) > maxWaitTime {
			log.Warn().Msg("Timeout waiting for pending request queue to drain, proceeding with snapshot")
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	crController.IsDoingSnapshot.Store(true)

	snapshotRequest := &protos.CreateSnapshotRequest{
		ServiceName:   config.GetServiceName(),
		RegistryName:  config.GetRegistryName(),
		Namespace:     config.GetNamespace(),
		LatestRequest: config.GetLatestRequestNumber(),
	}

	log.Info().Msg("Connecting to gRPC server at " + config.GetDaemonGrpcUrl())

	// Create connection with timeout
	connCtx, connCancel := context.WithTimeout(ctx, 10*time.Second)
	defer connCancel()

	conn, err := grpc.NewClient(config.GetDaemonGrpcUrl(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Err(err).Msg("failed to connect to gRPC server")
		releaseSnapshotLocks()
		return
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

	// Use timeout context for the Create call
	response, err := c.Create(connCtx, snapshotRequest)
	if err != nil {
		log.Err(err).Msg("failed to create snapshot")
		releaseSnapshotLocks()
		return
	}
	if response.GetResponse() != true {
		log.Error().Msg("failed to create snapshot: " + response.GetError())
		releaseSnapshotLocks()
		return
	}

	log.Info().Msg("Snapshot request sent successfully, waiting for Reply from daemon...")
	// Note: The locks will be released when Reply() is called by k8s-cr-daemon
}
