// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// GitEntry represents a git tree
type GitEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

// GitEntryWithCommit represents a git tree
type GitEntryWithCommit struct {
	Path   string      `json:"path"`
	Mode   string      `json:"mode"`
	Type   string      `json:"type"`
	Size   int64       `json:"size"`
	SHA    string      `json:"sha"`
	URL    string      `json:"url"`
	Commit interface{} `json:"commit"`
}

// GitTreeResponse returns a git tree
type GitTreeResponse struct {
	SHA        string     `json:"sha"`
	URL        string     `json:"url"`
	Entries    []GitEntry `json:"tree"`
	Truncated  bool       `json:"truncated"`
	Page       int        `json:"page"`
	TotalCount int        `json:"total_count"`
}

// GitTreeWithCommitsResponse returns a git tree
type GitTreeWithCommitsResponse struct {
	SHA          string               `json:"sha"`
	URL          string               `json:"url"`
	Entries      []GitEntryWithCommit `json:"tree"`
	Truncated    bool                 `json:"truncated"`
	Page         int                  `json:"page"`
	TotalCount   int                  `json:"total_count"`
	LatestCommit interface{}          `json:"latest_commit"`
}
