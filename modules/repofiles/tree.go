// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

// GetTreeBySHA get the GitTreeResponse of a repository using a sha hash.
func GetTreeBySHA(repo *models.Repository, sha string, page, perPage int, recursive bool) (*api.GitTreeResponse, error) {
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()
	gitTree, err := gitRepo.GetTree(sha)
	if err != nil || gitTree == nil {
		return nil, models.ErrSHANotFound{
			SHA: sha,
		}
	}
	tree := new(api.GitTreeResponse)
	tree.SHA = gitTree.ResolvedID.String()
	tree.URL = repo.APIURL() + "/git/trees/" + tree.SHA
	var entries git.Entries
	if recursive {
		entries, err = gitTree.ListEntriesRecursive()
	} else {
		entries, err = gitTree.ListEntries()
	}
	if err != nil {
		return nil, err
	}
	apiURL := repo.APIURL()
	apiURLLen := len(apiURL)

	// 51 is len(sha1) + len("/git/blobs/"). 40 + 11.
	blobURL := make([]byte, apiURLLen+51)
	copy(blobURL, apiURL)
	copy(blobURL[apiURLLen:], "/git/blobs/")

	// 51 is len(sha1) + len("/git/trees/"). 40 + 11.
	treeURL := make([]byte, apiURLLen+51)
	copy(treeURL, apiURL)
	copy(treeURL[apiURLLen:], "/git/trees/")

	// 40 is the size of the sha1 hash in hexadecimal format.
	copyPos := len(treeURL) - 40

	if perPage <= 0 || perPage > setting.API.DefaultGitTreesPerPage {
		perPage = setting.API.DefaultGitTreesPerPage
	}
	if page <= 0 {
		page = 1
	}
	tree.Page = page
	tree.TotalCount = len(entries)
	rangeStart := perPage * (page - 1)
	if rangeStart >= len(entries) {
		return tree, nil
	}
	var rangeEnd int
	if len(entries) > perPage {
		tree.Truncated = true
	}
	if rangeStart+perPage < len(entries) {
		rangeEnd = rangeStart + perPage
	} else {
		rangeEnd = len(entries)
	}
	tree.Entries = make([]api.GitEntry, rangeEnd-rangeStart)
	for e := rangeStart; e < rangeEnd; e++ {
		i := e - rangeStart

		tree.Entries[i].Path = entries[e].Name()
		tree.Entries[i].Mode = fmt.Sprintf("%06o", entries[e].Mode())
		tree.Entries[i].Type = entries[e].Type()
		tree.Entries[i].Size = entries[e].Size()
		tree.Entries[i].SHA = entries[e].ID.String()

		if entries[e].IsDir() {
			copy(treeURL[copyPos:], entries[e].ID.String())
			tree.Entries[i].URL = string(treeURL)
		} else {
			copy(blobURL[copyPos:], entries[e].ID.String())
			tree.Entries[i].URL = string(blobURL)
		}
	}
	return tree, nil
}

// GetTreeWithCommitsBySHA get the GitTreeWithCommitsResponse of a repository using a sha hash.
func GetTreeWithCommitsBySHA(repo *models.Repository, currentCommit *git.Commit, commitsCount int64, treePath, sha string, page, perPage int, recursive bool) (*api.GitTreeWithCommitsResponse, error) {
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()
	gitTree, err := gitRepo.GetTree(sha)
	if err != nil || gitTree == nil {
		return nil, models.ErrSHANotFound{
			SHA: sha,
		}
	}
	tree := new(api.GitTreeWithCommitsResponse)
	tree.SHA = gitTree.ResolvedID.String()
	tree.URL = repo.APIURL() + "/git/trees/" + tree.SHA
	var entries git.Entries
	if recursive {
		entries, err = gitTree.ListEntriesRecursive()
	} else {
		entries, err = gitTree.ListEntries()
	}
	if err != nil {
		return nil, err
	}
	entries.CustomSort(base.NaturalSortLess)

	var c git.LastCommitCache
	if setting.CacheService.LastCommit.Enabled && commitsCount >= setting.CacheService.LastCommit.CommitsCount {
		c = cache.NewLastCommitCache(repo.FullName(), gitRepo, int64(setting.CacheService.LastCommit.TTL.Seconds()))
	}

	treeCommits, latestCommit, err := entries.GetCommitsInfo(currentCommit, treePath, c)
	if err != nil {
		return nil, err
	}

	userCache := make(map[string]*models.User)
	tree.LatestCommit, _ = toCommitWithFullMessages(repo, latestCommit, userCache)

	commitCache := map[string]interface{}{}
	for i := range treeCommits {
		switch tc := treeCommits[i][1].(type) {
		case *git.SubModuleFile:
			commitCache[treeCommits[i][0].(*git.TreeEntry).Name()], _ = toCommitWithFullMessages(repo, tc.Commit, userCache)
		case *git.Commit:
			commitCache[treeCommits[i][0].(*git.TreeEntry).Name()], _ = toCommitWithFullMessages(repo, tc, userCache)
		default:
			commitCache[treeCommits[i][0].(*git.TreeEntry).Name()] = treeCommits[i][1]
		}
	}

	apiURL := repo.APIURL()
	apiURLLen := len(apiURL)

	// 51 is len(sha1) + len("/git/blobs/"). 40 + 11.
	blobURL := make([]byte, apiURLLen+51)
	copy(blobURL, apiURL)
	copy(blobURL[apiURLLen:], "/git/blobs/")

	// 51 is len(sha1) + len("/git/trees/"). 40 + 11.
	treeURL := make([]byte, apiURLLen+51)
	copy(treeURL, apiURL)
	copy(treeURL[apiURLLen:], "/git/trees/")

	// 40 is the size of the sha1 hash in hexadecimal format.
	copyPos := len(treeURL) - 40

	if perPage <= 0 || perPage > setting.API.DefaultGitTreesPerPage {
		perPage = setting.API.DefaultGitTreesPerPage
	}
	if page <= 0 {
		page = 1
	}
	tree.Page = page
	tree.TotalCount = len(entries)
	rangeStart := perPage * (page - 1)
	if rangeStart >= len(entries) {
		return tree, nil
	}
	var rangeEnd int
	if len(entries) > perPage {
		tree.Truncated = true
	}
	if rangeStart+perPage < len(entries) {
		rangeEnd = rangeStart + perPage
	} else {
		rangeEnd = len(entries)
	}
	tree.Entries = make([]api.GitEntryWithCommit, rangeEnd-rangeStart)
	for e := rangeStart; e < rangeEnd; e++ {
		i := e - rangeStart

		tree.Entries[i].Path = entries[e].Name()
		tree.Entries[i].Mode = fmt.Sprintf("%06o", entries[e].Mode())
		tree.Entries[i].Type = entries[e].Type()
		tree.Entries[i].Size = entries[e].Size()
		tree.Entries[i].SHA = entries[e].ID.String()
		tree.Entries[i].Commit = commitCache[entries[e].Name()]

		if entries[e].IsDir() {
			copy(treeURL[copyPos:], entries[e].ID.String())
			tree.Entries[i].URL = string(treeURL)
		} else {
			copy(blobURL[copyPos:], entries[e].ID.String())
			tree.Entries[i].URL = string(blobURL)
		}
	}
	return tree, nil
}

func toCommitWithFullMessages(repo *models.Repository, commit *git.Commit, userCache map[string]*models.User) (*api.Commit, error) {

	var apiAuthor, apiCommitter *api.User

	// Retrieve author and committer information

	var cacheAuthor *models.User
	var ok bool
	if userCache == nil {
		cacheAuthor = ((*models.User)(nil))
		ok = false
	} else {
		cacheAuthor, ok = userCache[commit.Author.Email]
	}

	if ok {
		apiAuthor = cacheAuthor.APIFormat()
	} else {
		author, err := models.GetUserByEmail(commit.Author.Email)
		if err != nil && !models.IsErrUserNotExist(err) {
			return nil, err
		} else if err == nil {
			apiAuthor = author.APIFormat()
			if userCache != nil {
				userCache[commit.Author.Email] = author
			}
		}
	}

	var cacheCommitter *models.User
	if userCache == nil {
		cacheCommitter = ((*models.User)(nil))
		ok = false
	} else {
		cacheCommitter, ok = userCache[commit.Committer.Email]
	}

	if ok {
		apiCommitter = cacheCommitter.APIFormat()
	} else {
		committer, err := models.GetUserByEmail(commit.Committer.Email)
		if err != nil && !models.IsErrUserNotExist(err) {
			return nil, err
		} else if err == nil {
			apiCommitter = committer.APIFormat()
			if userCache != nil {
				userCache[commit.Committer.Email] = committer
			}
		}
	}

	// Retrieve parent(s) of the commit
	apiParents := make([]*api.CommitMeta, commit.ParentCount())
	for i := 0; i < commit.ParentCount(); i++ {
		sha, _ := commit.ParentID(i)
		apiParents[i] = &api.CommitMeta{
			URL: repo.APIURL() + "/git/commits/" + sha.String(),
			SHA: sha.String(),
		}
	}

	return &api.Commit{
		CommitMeta: &api.CommitMeta{
			URL: repo.APIURL() + "/git/commits/" + commit.ID.String(),
			SHA: commit.ID.String(),
		},
		HTMLURL: repo.HTMLURL() + "/commit/" + commit.ID.String(),
		RepoCommit: &api.RepoCommit{
			URL: repo.APIURL() + "/git/commits/" + commit.ID.String(),
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  commit.Committer.Name,
					Email: commit.Committer.Email,
				},
				Date: commit.Author.When.Format(time.RFC3339),
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  commit.Committer.Name,
					Email: commit.Committer.Email,
				},
				Date: commit.Committer.When.Format(time.RFC3339),
			},
			Message: commit.String(),
			Tree: &api.CommitMeta{
				URL: repo.APIURL() + "/git/trees/" + commit.ID.String(),
				SHA: commit.ID.String(),
			},
		},
		Author:    apiAuthor,
		Committer: apiCommitter,
		Parents:   apiParents,
	}, nil
}
