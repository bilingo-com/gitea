// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Blame represents show who modified each part of the file
type Blame struct {
	FileName   string       `json:"file_name"`
	FileSize   int64        `json:"file_size"`
	BlameParts []*BlamePart `json:"blame_parts"`
}

type BlamePart struct {
	SHA   string   `json:"sha"`
	Lines []string `json:"lines"`
	*Commit
}
