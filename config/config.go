package config

import (
	"os"
	"strconv"
	"sync"
)

var SnapshotLock = &sync.Mutex{}
var IsSnapshotBeingTaken = false

// VerifyEnvVars Env vars:
// - APPLICATION_URL: Full application URL, with port
// - INTERCEPTOR_PORT: Port to listen to
// - HEARTBEAT_ENABLED: Enable or disable the heartbeat
// - CHECKPOINT_ENABLED: Enable or disable the checkpoint
func VerifyEnvVars() {
	applicationUrl, ok := os.LookupEnv("APPLICATION_URL")
	if !ok {
		panic("Couldn't find the INTERCEPTOR_PORT variable")
	}

	if applicationUrl == "" {
		panic("APPLICATION_URL can't be empty")
	}

	podName, ok := os.LookupEnv("POD_NAME")
	if !ok {
		panic("Couldn't find the POD_NAME variable")
	}

	if podName == "" {
		panic("POD_NAME can't be empty")
	}

	interceptorPort, ok := os.LookupEnv("INTERCEPTOR_PORT")

	if !ok {
		panic("Couldn't find the INTERCEPTOR_PORT variable")
	}

	if interceptorPort == "" {
		panic("INTERCEPTOR_PORT can't be empty")
	}

	_, err := strconv.Atoi(interceptorPort)
	if err != nil {
		panic("INTERCEPTOR_PORT must be a number")
	}

	heartBeatEnabled, ok := os.LookupEnv("HEARTBEAT_ENABLED")
	if !ok {
		panic("Couldn't find the INTERCEPTOR_PORT variable")
	}

	if _, err := strconv.ParseBool(heartBeatEnabled); err != nil {
		panic("HEARTBEAT_ENABLED must be a boolean")
	}

	checkpointEnabled, ok := os.LookupEnv("CHECKPOINT_ENABLED")
	if !ok {
		panic("Couldn't find the CHECKPOINT_ENABLED variable")
	}

	if _, err := strconv.ParseBool(checkpointEnabled); err != nil {
		panic("CHECKPOINT_ENABLED must be a boolean")
	}

	// @TODO: add REGISTRY_URL

	nameSpace, ok := os.LookupEnv("NAMESPACE")
	if !ok {
		panic("Couldn't find the NAMESPACE variable")
	}
	if nameSpace == "" {
		panic("NAMESPACE can't be empty")
	}

	serviceName, ok := os.LookupEnv("SERVICE_NAME")
	if !ok {
		panic("Couldn't find the SERVICE_NAME variable")
	}
	if serviceName == "" {
		panic("SERVICE_NAME can't be empty")
	}

	registryName, ok := os.LookupEnv("REGISTRY_NAME")
	if !ok {
		panic("Couldn't find the REGISTRY_NAME variable")
	}
	if registryName == "" {
		panic("REGISTRY_NAME can't be empty")
	}

}

func GetApplicationURL() string {
	return os.Getenv("APPLICATION_URL")
}

func GetInterceptorPort() string {
	interceptorPort := os.Getenv("INTERCEPTOR_PORT")
	if interceptorPort[0] != ':' {
		interceptorPort = ":" + interceptorPort
	}
	return interceptorPort
}

func GetNamespace() string {
	return os.Getenv("NAMESPACE")
}

func GetServiceName() string {
	return os.Getenv("SERVICE_NAME")
}

func GetRegistryName() string {
	return os.Getenv("REGISTRY_NAME")
}

func GetHeartBeatEnabled() bool {
	heartBeatEnabled, err := strconv.ParseBool(os.Getenv("HEARTBEAT_ENABLED"))
	if err != nil {
		panic("HEARTBEAT_ENABLED must be a boolean")
	}
	return heartBeatEnabled
}

func GetCheckpointEnabled() bool {
	checkpointEnabled, err := strconv.ParseBool(os.Getenv("CHECKPOINT_ENABLED"))
	if err != nil {
		panic("CHECKPOINT_ENABLED must be a boolean")
	}
	return checkpointEnabled
}

func GetEnableTrace() bool {
	enableTrace, err := strconv.ParseBool(os.Getenv("ENABLE_TRACE"))
	if err != nil {
		return false
	}
	return enableTrace
}

func GetPodName() string {
	return os.Getenv("POD_NAME")
}
