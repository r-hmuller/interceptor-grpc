package snapshotter

import (
	"context"
	"interceptor-grpc/config"
	"interceptor-grpc/protos"
)

func GenerateSnapshots() {

}

func generateSnapshot(ctx context.Context) {
	_ = &protos.CreateSnapshotRequest{
		ServiceName:   config.GetServiceName(),
		RegistryName:  config.GetRegistryName(),
		Namespace:     config.GetNamespace(),
		LatestRequest: config.GetLatestRequestNumber(),
	}

	//Aqui preciso enviar via gRPC para o servidor de snapshot
}
