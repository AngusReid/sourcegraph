package worker

import (
	"fmt"
	"io"
	"time"

	"sourcegraph.com/sqs/pbtypes"

	"golang.org/x/net/context"
	"gopkg.in/inconshreveable/log15.v2"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/worker/builder"
)

// startBuild starts and monitors a single build. It manages the
// build's state on the Sourcegraph server.
func startBuild(ctx context.Context, build *sourcegraph.Build) {
	done := startHeartbeat(ctx, build.Spec())
	defer done()

	start := time.Now()

	log15.Info("Starting build", "build", build.Spec().IDString())
	_, err := sourcegraph.NewClientFromContext(ctx).Builds.Update(ctx, &sourcegraph.BuildsUpdateOp{
		Build: build.Spec(),
		Info:  sourcegraph.BuildUpdate{StartedAt: now()},
	})
	if err != nil {
		log15.Error("Updating build starting state failed", "build", build.Spec(), "err", err)
		return
	}

	// Configure build.
	builder, err := configureBuild(ctx, build)
	if err != nil {
		log15.Error("Configuring build failed", "build", build.Spec(), "err", err)
		return
	}

	// Run build.
	execErr := builder.Exec(ctx)
	if execErr == nil {
		log15.Info("Build succeeded", "build", build.Spec().IDString(), "time", time.Since(start))
	} else {
		log15.Info("Build failed", "build", build.Spec().IDString(), "time", time.Since(start), "err", execErr)
	}

	_, err = sourcegraph.NewClientFromContext(ctx).Builds.Update(ctx, &sourcegraph.BuildsUpdateOp{
		Build: build.Spec(),
		Info: sourcegraph.BuildUpdate{
			Success: execErr == nil,
			Failure: execErr != nil,
			EndedAt: now(),
		},
	})
	if err != nil {
		log15.Error("Updating build final state failed", "build", build.Spec(), "err", err)
	}
}

// taskState manages the state of a task stored on the Sourcegraph
// server. It implements builder.TaskState.
type taskState struct {
	task sourcegraph.TaskSpec

	// log is where task logs are written. Internal errors
	// encountered by the builder are not written to w but are
	// returned as errors from its methods.
	log io.WriteCloser
}

// Start implements builder.TaskState.
func (s taskState) Start(ctx context.Context) error {
	_, err := sourcegraph.NewClientFromContext(ctx).Builds.UpdateTask(ctx, &sourcegraph.BuildsUpdateTaskOp{
		Task: s.task,
		Info: sourcegraph.TaskUpdate{
			StartedAt: now(),
		},
	})
	if err != nil {
		fmt.Fprintf(s.log, "Error starting task: %s\n", err)
	}
	return err
}

// Skip implements builder.TaskState.
func (s taskState) Skip(ctx context.Context) error {
	_, err := sourcegraph.NewClientFromContext(ctx).Builds.UpdateTask(ctx, &sourcegraph.BuildsUpdateTaskOp{
		Task: s.task,
		Info: sourcegraph.TaskUpdate{
			Skipped: true,
			EndedAt: now(),
		},
	})
	if err != nil {
		fmt.Fprintf(s.log, "Error marking task as skipped: %s\n", err)
	}
	return err
}

// Warnings implements builder.TaskState.
func (s taskState) Warnings(ctx context.Context) error {
	_, err := sourcegraph.NewClientFromContext(ctx).Builds.UpdateTask(ctx, &sourcegraph.BuildsUpdateTaskOp{
		Task: s.task,
		Info: sourcegraph.TaskUpdate{Warnings: true},
	})
	if err != nil {
		fmt.Fprintf(s.log, "Error marking task as having warnings: %s\n", err)
	}
	return err
}

// End implements builder.TaskState.
func (s taskState) End(ctx context.Context, execErr error) error {
	defer s.log.Close()

	_, err := sourcegraph.NewClientFromContext(ctx).Builds.UpdateTask(ctx, &sourcegraph.BuildsUpdateTaskOp{
		Task: s.task,
		Info: sourcegraph.TaskUpdate{
			Success: execErr == nil,
			Failure: execErr != nil,
			EndedAt: now(),
		},
	})
	if err != nil {
		fmt.Fprintf(s.log, "Error ending build task: %s\n", err)
	}
	return err
}

// CreateSubtask implements builder.TaskState.
func (s taskState) CreateSubtask(ctx context.Context, label string) (builder.TaskState, error) {
	tasks, err := sourcegraph.NewClientFromContext(ctx).Builds.CreateTasks(ctx, &sourcegraph.BuildsCreateTasksOp{
		Build: s.task.Build,
		Tasks: []*sourcegraph.BuildTask{
			{Label: label, ParentID: s.task.ID},
		},
	})
	if err != nil {
		fmt.Fprintf(s.log, "Error creating subtask with label %q: %s\n", label, err)
		return nil, err
	}
	subtask := tasks.BuildTasks[0].Spec()
	return &taskState{
		task: subtask,
		log:  newLogger(subtask),
	}, nil
}

func (s taskState) Log() io.Writer { return s.log }

func (s taskState) String() string { return s.task.IDString() }

func now() *pbtypes.Timestamp {
	now := pbtypes.NewTimestamp(time.Now())
	return &now
}
