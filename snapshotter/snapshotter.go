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

var snapshotStartTime time.Time
var replyTimeout = 4 * time.Minute

func GenerateSnapshots(ctx context.Context) {
	tick := time.Tick(time.Duration(config.GetCheckpointInterval()) * time.Second)
	maxSnapshotDuration := 5 * time.Minute
	for range tick {
		// Lock before checking to prevent race condition
		config.SnapshotLock.Lock()
		if config.IsSnapshotBeingTaken {
			elapsed := time.Since(snapshotStartTime)
			if elapsed > maxSnapshotDuration {
				config.SnapshotLock.Unlock()
				log.Warn().
					Dur("elapsed", elapsed).
					Dur("max_duration", maxSnapshotDuration).
					Msg("Snapshot has been in progress for too long, forcing lock release")
				releaseSnapshotLocks()
				continue
			}
			config.SnapshotLock.Unlock()
			continue
		}
		config.IsSnapshotBeingTaken = true
		snapshotStartTime = time.Now()
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
	// Block new requests first
	crController.IsDoingSnapshot.Store(true)

	// Wait for all in-flight HTTP requests to complete
	waitDone := make(chan struct{})
	go func() {
		crController.InFlightRequests.Wait()
		close(waitDone)
	}()

	maxWaitTime := 30 * time.Second
	select {
	case <-waitDone:
	case <-time.After(maxWaitTime):
		log.Warn().Msg("Timeout waiting for in-flight requests, proceeding with snapshot")
	}

	snapshotRequest := &protos.CreateSnapshotRequest{
		ServiceName:   config.GetServiceName(),
		RegistryName:  config.GetRegistryName(),
		Namespace:     config.GetNamespace(),
		LatestRequest: config.GetLatestRequestNumber(),
	}

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

	// Safety net: release locks if Reply() is not received in time.
	// Without this, a daemon failure after Create() leaves the system blocked indefinitely.
	go func() {
		time.Sleep(replyTimeout)
		if crController.IsDoingSnapshot.Load() {
			log.Warn().
				Dur("timeout", replyTimeout).
				Msg("Reply() not received in time after successful Create(), forcing lock release")
			releaseSnapshotLocks()
		}
	}()
}
