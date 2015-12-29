package httpapi

import (
	"reflect"
	"testing"

	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
)

func TestBuild(t *testing.T) {
	c, mock := newTest()

	wantBuild := &sourcegraph.Build{CommitID: "ASD", Attempt: 123, Repo: "r/r"}

	calledGet := mock.Builds.MockGet_Return(t, wantBuild)

	var build *sourcegraph.Build
	if err := c.GetJSON("/repos/r/r/.builds/ASD/123", &build); err != nil {
		t.Logf("%#v", build)
		t.Fatal(err)
	}
	if !reflect.DeepEqual(build, wantBuild) {
		t.Errorf("got %+v, want %+v", build, wantBuild)
	}
	if !*calledGet {
		t.Error("!calledGet")
	}
}

func TestBuilds(t *testing.T) {
	c, mock := newTest()

	wantBuilds := &sourcegraph.BuildList{Builds: []*sourcegraph.Build{{Attempt: 123, CommitID: "ASD", Repo: "r/r"}}}

	calledList := mock.Builds.MockList(t, wantBuilds.Builds...)

	var builds *sourcegraph.BuildList
	if err := c.GetJSON("/builds", &builds); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(builds, wantBuilds) {
		t.Errorf("got %+v, want %+v", builds, wantBuilds)
	}
	if !*calledList {
		t.Error("!calledList")
	}
}

func TestBuildTasks(t *testing.T) {
	c, mock := newTest()

	wantTasks := &sourcegraph.BuildTaskList{BuildTasks: []*sourcegraph.BuildTask{{TaskID: 123}}}

	calledListBuildTasks := mock.Builds.MockListBuildTasks(t, wantTasks.BuildTasks...)

	var tasks *sourcegraph.BuildTaskList
	if err := c.GetJSON("/repos/r/.builds/abc/123/.tasks", &tasks); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tasks, wantTasks) {
		t.Errorf("got %+v, want %+v", tasks, wantTasks)
	}
	if !*calledListBuildTasks {
		t.Error("!calledListBuildTasks")
	}
}
