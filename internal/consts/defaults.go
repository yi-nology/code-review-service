package consts

import "time"

const (
	DefaultBinary          = "ocr"
	DefaultAudience        = "human"
	DefaultSeverity        = "major"
	DefaultConcurrency     = 2
	DefaultMaxGitProcs     = 4
	DefaultTimeoutMinutes  = 10
	DefaultMaxRetries      = 1
	DefaultMaxComments     = 20
	DefaultFailOn          = "critical"
	DefaultWorkerStaleAfter = 15 * time.Minute
	DefaultProgressRetention = 10 * time.Minute
	DefaultViewerAddr      = "localhost:5483"
	DefaultViewerStopTimeout = 3 * time.Second
)
