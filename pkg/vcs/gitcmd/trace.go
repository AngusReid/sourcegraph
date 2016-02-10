package gitcmd

import (
	"fmt"
	"strings"
	"time"

	"sourcegraph.com/sourcegraph/appdash"
)

func init() { appdash.RegisterEvent(GitEvent{}) }

type GitEvent struct {
	Name, Args string
	StartTime  time.Time
	EndTime    time.Time
}

func (GitEvent) Schema() string { return "Git" }

func (e GitEvent) Start() time.Time { return e.StartTime }
func (e GitEvent) End() time.Time   { return e.EndTime }

func (r *Repository) trace(start time.Time, name string, args ...interface{}) {
	if r.AppdashRec != nil {
		argStrs := make([]string, len(args))
		for i, arg := range args {
			argStrs[i] = fmt.Sprintf("%#v", arg)
		}
		r.AppdashRec.Child().Event(GitEvent{
			Name:      "git." + name,
			Args:      strings.Join(argStrs, ", "),
			StartTime: start,
			EndTime:   time.Now(),
		})
	}
}
