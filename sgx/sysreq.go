package sgx

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kr/text"

	"src.sourcegraph.com/sourcegraph/pkg/sysreq"
	"src.sourcegraph.com/sourcegraph/sgx/client"

	"golang.org/x/net/context"
)

const skipSysReqsEnvVar = "SRC_SKIP_REQS"

// skippedSysReqs returns a list of sysreq names to skip (e.g.,
// "Docker").
func skippedSysReqs() []string {
	return strings.Fields(os.Getenv(skipSysReqsEnvVar))
}

// checkSysReqs uses package sysreq to check for the presence of
// system requirements. If any are missing, it prints a message to
// w and returns a non-nil error.
func checkSysReqs(ctx context.Context, w io.Writer) error {
	wrap := func(s string) string {
		const indent = "\t\t"
		return strings.TrimPrefix(text.Indent(text.Wrap(s, 72), "\t\t"), indent)
	}

	var failed []string
	for _, st := range sysreq.Check(client.Ctx, skippedSysReqs()) {
		if st.Failed() {
			failed = append(failed, st.Name)

			fmt.Fprint(w, redbg(" !!!!! "))
			fmt.Fprintf(w, bold(red(" %s is required\n")), st.Name)
			if st.Problem != "" {
				fmt.Fprint(w, bold(red("\tProblem: ")))
				fmt.Fprintln(w, red(wrap(st.Problem)))
			}
			if st.Err != nil {
				fmt.Fprint(w, bold(red("\tError: ")))
				fmt.Fprintln(w, red(wrap(st.Err.Error())))
			}
			if st.Fix != "" {
				fmt.Fprint(w, bold(green("\tPossible fix: ")))
				fmt.Fprintln(w, green(wrap(st.Fix)))
			}
			fmt.Fprintln(w, "\t"+cyan(wrap(fmt.Sprintf("Skip this check by setting the env var %s=%q (separate multiple entries with spaces). Note: Sourcegraph may not function properly without %s.", skipSysReqsEnvVar, st.Name, st.Name))))
		}
	}

	if failed != nil {
		return fmt.Errorf("system requirement checks failed: %v (see above for more information)", failed)
	}
	return nil
}
