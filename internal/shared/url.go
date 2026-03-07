// Package shared contains functionality that is used across module depths.
//
//nolint:revive
package shared

import "net/url"

// MustJoinURLPath is a wrapper around url.JoinPath which panics if it returns an error.
func MustJoinURLPath(base string, path ...string) string {
	result, err := url.JoinPath(base, path...)
	if err != nil {
		panic(err)
	}

	return result
}
