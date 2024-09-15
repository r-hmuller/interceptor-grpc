package crController

import "sync/atomic"

var IsRunningPendingRequestQueue atomic.Bool
var IsDoingSnapshot atomic.Bool
var IsRestoringSnapshot atomic.Bool

func IsUnavailable() bool {
	return IsRunningPendingRequestQueue.Load() || IsDoingSnapshot.Load() || IsRestoringSnapshot.Load()
}
