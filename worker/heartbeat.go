package worker

import (
	"log"
	"time"

	"golang.org/x/net/context"

	"sourcegraph.com/sqs/pbtypes"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
)

// a reasonable guess; TODO(sqs): check the server's actual setting
const serverHeartbeatInterval = 15 * time.Second

// startHeartbeat spawns a background goroutine that sends heartbeats to the server until done is called.
func startHeartbeat(ctx context.Context, build sourcegraph.BuildSpec) (done func()) {
	quitCh := make(chan struct{})

	cl := sourcegraph.NewClientFromContext(ctx)
	go func() {
		t := time.NewTicker(serverHeartbeatInterval)
		for {
			select {
			case _, ok := <-t.C:
				if !ok {
					return
				}
				now := pbtypes.NewTimestamp(time.Now())
				_, err := cl.Builds.Update(ctx, &sourcegraph.BuildsUpdateOp{Build: build, Info: sourcegraph.BuildUpdate{HeartbeatAt: &now}})
				if err != nil {
					log.Printf("Worker heartbeat failed in Builds.Update call for build %+v: %s.", build, err)
					return
				}
			case <-quitCh:
				t.Stop()
				return
			}
		}
	}()

	return func() {
		close(quitCh)
	}
}
