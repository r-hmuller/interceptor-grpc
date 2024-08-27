package config

import (
	"os"
	"strconv"
	"sync"
)

var snapshotLock = &sync.Mutex{}
var isSnapshotBeingTaken = false

// Env vars:
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
