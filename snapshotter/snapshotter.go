package snapshotter

import (
	"context"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"interceptor-grpc/config"
	"interceptor-grpc/protos"
	"time"
)

func GenerateSnapshots(ctx context.Context) {
	tick := time.Tick(time.Duration(config.GetCheckpointInterval()) * time.Second)
	for range tick {
		if config.IsSnapshotBeingTaken {
			continue
		}
		config.SnapshotLock.Lock()
		config.IsSnapshotBeingTaken = true
		config.SnapshotLock.Unlock()

		generateSnapshot(ctx)
	}
}

func generateSnapshot(ctx context.Context) {
	_ = &protos.CreateSnapshotRequest{
		ServiceName:   config.GetServiceName(),
		RegistryName:  config.GetRegistryName(),
		Namespace:     config.GetNamespace(),
		LatestRequest: config.GetLatestRequestNumber(),
	}

	//Aqui preciso enviar via gRPC para o servidor de snapshot

	conn, err := grpc.NewClient(config.GetDaemonGrpcUrl(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Err(err).Msg("failed to connect to gRPC server at localhost:50051")
	}
	defer conn.Close()

	c := protos.NewSnapshotRPCServiceClient(conn)
	response, err := c.Create(ctx, &protos.CreateSnapshotRequest{})
	if err != nil {
		log.Err(err).Msg("failed to create snapshot")
	}
	if response.GetResponse() != true {
		log.Error().Msg("failed to create snapshot: " + response.GetError())
	}
}
