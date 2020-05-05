// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	goContext "context"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
)

func RefBlameByBranch(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/blame/branch/{branchAndFilePath} repository repoRefBlameByBranch
	// ---
	// summary: Get a blame from a repository by branch
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: branchAndFilePath
	//   in: path
	//   description: a <branch>/<filePath>
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Blame"
	//   "404":
	//     "$ref": "#/responses/notFound"
	refBlame(ctx)
}

// RefBlameByTag get blame info by tag
func RefBlameByTag(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/blame/tag/{tagAndFilePath} repository repoRefBlameByTag
	// ---
	// summary: Get a blame from a repository by tag
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: tagAndFilePath
	//   in: path
	//   description: a <tag>/<filePath>
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Blame"
	//   "404":
	//     "$ref": "#/responses/notFound"
	refBlame(ctx)
}

// RefBlameByCommit get blame info by commit
func RefBlameByCommit(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/blame/commit/{commitAndFilePath} repository repoRefBlameByCommit
	// ---
	// summary: Get a blame from a repository by commit.
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: commitAndFilePath
	//   in: path
	//   description: a <commit>/<filePath>
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Blame"
	//   "404":
	//     "$ref": "#/responses/notFound"
	refBlame(ctx)
}

func refBlame(ctx *context.APIContext) {
	blameResp := new(api.Blame)

	fileName := ctx.Repo.TreePath
	if len(fileName) == 0 {
		ctx.NotFoundOrServerError("Blame FileName", git.IsErrNotExist, nil)
		return
	}

	userName := ctx.Repo.Owner.Name
	repoName := ctx.Repo.Repository.Name
	commitID := ctx.Repo.CommitID

	commit, err := ctx.Repo.GitRepo.GetCommit(commitID)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.GitRepo.GetCommit", git.IsErrNotExist, err)
		if git.IsErrNotExist(err) {
			ctx.NotFound("Repo.GitRepo.GetCommit", err)
		} else {
			ctx.ServerError("Repo.GitRepo.GetCommit", err)
		}
		return
	}
	if len(commitID) != 40 {
		commitID = commit.ID.String()
	}

	// Get current entry user currently looking at.
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.GetTreeEntryByPath", git.IsErrNotExist, err)
		return
	}

	blob := entry.Blob()

	blameResp.FileSize = blob.Size()
	blameResp.FileName = blob.Name()

	blameReader, err := git.CreateBlameReader(goContext.Background(), models.RepoPath(userName, repoName), commitID, fileName)
	if err != nil {
		ctx.NotFound("CreateBlameReader", err)
		return
	}
	defer blameReader.Close()

	blameParts := make([]git.BlamePart, 0)

	for {
		blamePart, err := blameReader.NextPart()
		if err != nil {
			ctx.NotFound("NextPart", err)
			return
		}
		if blamePart == nil {
			break
		}
		blameParts = append(blameParts, *blamePart)
	}

	blameResp.BlameParts = make([]*api.BlamePart, 0, len(blameParts))
	tempCommit := make(map[string]*api.Commit)
	for _, part := range blameParts {
		blame := &api.BlamePart{SHA: part.Sha}
		var (
			ct *api.Commit
			ok bool
		)
		if ct, ok = tempCommit[part.Sha]; !ok {
			gitCt, err := ctx.Repo.GitRepo.GetCommit(part.Sha)
			if err != nil {
				if git.IsErrNotExist(err) {
					ctx.NotFound("Repo.GitRepo.GetCommit", err)
				} else {
					ctx.ServerError("Repo.GitRepo.GetCommit", err)
				}
				return
			}
			tempCommit[part.Sha], err = toCommit(ctx, ctx.Repo.Repository, gitCt, nil)
			if err != nil {
				ctx.ServerError("Repo.GitRepo.GetCommit", err)
				return
			}
			ct = tempCommit[part.Sha]
		}
		blame.Commit = ct
		blame.Lines = part.Lines
		blameResp.BlameParts = append(blameResp.BlameParts,
			blame)
	}

	ctx.JSON(http.StatusOK, blameResp)
}
