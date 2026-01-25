package heartbeat

import (
	"github.com/rs/zerolog/log"
	"interceptor-grpc/config"
	"interceptor-grpc/crController"
	"io"
	"net/http"
	"strings"
	"time"
)

func Monitor() {
	// This function should be called in a go routine
	// It should monitor the heartbeat of the interceptor
	// If the interceptor is not responding, it should restart the interceptor
	path := config.GetHeartBeatPath()
	applicationURL := strings.TrimRight(config.GetApplicationURL(), "/")
	fullPath := applicationURL + "/" + path
	// Make a request to the interceptor
	numberRequestsFailed := 0
	numberRequestsSuccess := 0

	tick := time.Tick(5 * time.Second)
	for range tick {
		resp, err := http.Get(fullPath)
		if err != nil {
			// handle error
		}
		defer resp.Body.Close()
		_, err = io.ReadAll(resp.Body)
		if resp.StatusCode > 299 {
			log.Error().Msgf("Heartbeat failed with status code %d", resp.StatusCode)
			numberRequestsFailed++
			numberRequestsSuccess = 0
		} else {
			numberRequestsSuccess++
			numberRequestsFailed = 0
		}
		if numberRequestsFailed > 5 {
			log.Error().Msg("Heartbeat failed more than 5 times, marking container as unavailable")
			crController.IsContainerUnavailable.Store(true)

		}
		if numberRequestsSuccess > 5 {
			log.Info().Msg("Heartbeat succeeded more than 5 times, marking container as available")
			crController.IsContainerUnavailable.Store(false)
		}
	}
}
