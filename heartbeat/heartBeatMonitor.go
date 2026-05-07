package heartbeat

import (
	"interceptor-grpc/config"
	"interceptor-grpc/crController"
	"io"
	"net/http"
	"strings"
	"time"
)

var hbClient = &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

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
		// #E: skip enquanto snapshot/restore esta acontecendo. CRIU congela o backend
		// durante o dump, fazendo /health retornar timeout/erro -- contar como falha
		// abriria o circuito falsamente.
		if crController.IsDoingSnapshot.Load() || crController.IsRestoringSnapshot.Load() {
			numberRequestsFailed = 0
			numberRequestsSuccess = 0
			continue
		}
		resp, err := hbClient.Get(fullPath)
		if err != nil {
			numberRequestsFailed++
			numberRequestsSuccess = 0
			if numberRequestsFailed > 5 {
				crController.IsContainerUnavailable.Store(true)
			}
			continue
		}
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode > 299 {
			numberRequestsFailed++
			numberRequestsSuccess = 0
		} else {
			numberRequestsSuccess++
			numberRequestsFailed = 0
		}
		if numberRequestsFailed > 5 {
			crController.IsContainerUnavailable.Store(true)
		}
		if numberRequestsSuccess > 5 {
			crController.IsContainerUnavailable.Store(false)
		}
	}
}
