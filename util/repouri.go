package util

import (
	"net/url"
	"regexp"
	"strings"
)

// RepoURIHost returns the host of the given repoURI, converted to lower case, or empty string on error.
func RepoURIHost(repoURI string) string {
	u, err := url.Parse("//" + repoURI)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}

var trackedRepoRe = regexp.MustCompile(`\b(github.com/kubernetes/kubernetes)\b`)

// GetTrackedRepo guesses which repo a request URL path is for. It only looks
// at a certain subset of repos for its guess.
func GetTrackedRepo(path string) string {
	m := trackedRepoRe.FindStringSubmatch(path)
	if len(m) == 0 {
		return "unknown"
	}
	return m[1]
}
