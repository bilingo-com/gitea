// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/routers/api/v1/utils"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
)

// ListPullRequests returns a list of all PRs
func ListPullRequests(ctx *context.APIContext, form api.ListPullRequestsOptions) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls repository repoListPullRequests
	// ---
	// summary: List a repo's pull requests
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
	// - name: state
	//   in: query
	//   description: "State of pull request: open or closed (optional)"
	//   type: string
	//   enum: [closed, open, all]
	// - name: sort
	//   in: query
	//   description: "Type of sort"
	//   type: string
	//   enum: [oldest, recentupdate, leastupdate, mostcomment, leastcomment, priority]
	// - name: milestone
	//   in: query
	//   description: "ID of the milestone"
	//   type: integer
	//   format: int64
	// - name: labels
	//   in: query
	//   description: "Label IDs"
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: integer
	//     format: int64
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullRequestList"

	listOptions := utils.GetListOptions(ctx)

	prs, maxResults, err := models.PullRequests(ctx.Repo.Repository.ID, &models.PullRequestsOptions{
		ListOptions: listOptions,
		State:       ctx.QueryTrim("state"),
		SortType:    ctx.QueryTrim("sort"),
		Labels:      ctx.QueryStrings("labels"),
		MilestoneID: ctx.QueryInt64("milestone"),
	})

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "PullRequests", err)
		return
	}

	apiPrs := make([]*api.PullRequest, len(prs))
	for i := range prs {
		if err = prs[i].LoadIssue(); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
			return
		}
		if err = prs[i].LoadAttributes(); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
			return
		}
		if err = prs[i].LoadBaseRepo(); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadBaseRepo", err)
			return
		}
		if err = prs[i].LoadHeadRepo(); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadHeadRepo", err)
			return
		}
		apiPrs[i] = convert.ToAPIPullRequest(prs[i])
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.Header().Set("X-Total-Count", fmt.Sprintf("%d", maxResults))
	ctx.JSON(http.StatusOK, &apiPrs)
}

// GetPullRequest returns a single PR based on index
func GetPullRequest(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index} repository repoGetPullRequest
	// ---
	// summary: Get a pull request
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
	// - name: index
	//   in: path
	//   description: index of the pull request to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullRequest"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if err = pr.LoadBaseRepo(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadBaseRepo", err)
		return
	}

	if err = pr.LoadHeadRepo(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadHeadRepo", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIPullRequest(pr))
}

// GetPullWithDiffAndPatchRawRequest returns a single PR based on index
func GetPullWithDiffAndPatchRawRequest(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/diffAndPatchRaw repository repoGetPullWithDiffAndPatchRawRequest
	// ---
	// summary: Get a pull request with diff and patch raw
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
	// - name: index
	//   in: path
	//   description: index of the pull request to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullRequestWithDiffAndPatchRaw"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if err = pr.LoadBaseRepo(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadBaseRepo", err)
		return
	}

	if err = pr.LoadHeadRepo(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadHeadRepo", err)
		return
	}

	prResp := convert.ToAPIPullRequestWithDiffAndPatchRaw(pr)

	buffer := &bytes.Buffer{}
	if err := pull_service.DownloadDiffOrPatch(pr, buffer, false); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadDiffRaw", err)
		return
	}
	if len(buffer.Bytes()) > MAX_DIFF_LEN {
		prResp.DiffRaw = string(buffer.Bytes()[:MAX_DIFF_LEN])
	} else {
		prResp.DiffRaw = buffer.String()
	}

	buffer.Reset()
	if err := pull_service.DownloadDiffOrPatch(pr, buffer, true); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadPatchRaw", err)
		return
	}
	if len(buffer.Bytes()) > MAX_PATCH_LEN {
		prResp.PatchRaw = string(buffer.Bytes()[:MAX_DIFF_LEN])
	} else {
		prResp.PatchRaw = buffer.String()
	}

	// Load commits
	compareInfo, err := ctx.Repo.GitRepo.GetCompareInfo(ctx.Repo.Repository.RepoPath(),
		pr.MergeBase, pr.HeadBranch)

	userCache := make(map[string]*models.User)

	apiCommits := make([]*api.Commit, compareInfo.Commits.Len())

	i := 0
	for commitPointer := compareInfo.Commits.Front(); commitPointer != nil; commitPointer = commitPointer.Next() {
		commit := commitPointer.Value.(*git.Commit)

		// Create json struct
		apiCommits[i], err = toCommitWithFullMessages(ctx, ctx.Repo.Repository, commit, userCache)
		if err != nil {
			ctx.ServerError("toCommit", err)
			return
		}

		i++
	}
	prResp.Commits = apiCommits
	prResp.CommitsCount = int64(compareInfo.Commits.Len())
	prResp.CommitsNumFiles = int64(compareInfo.NumFiles)

	ctx.JSON(http.StatusOK, prResp)
}

// CreatePullRequest does what it says
func CreatePullRequest(ctx *context.APIContext, form api.CreatePullRequestOption) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls repository repoCreatePullRequest
	// ---
	// summary: Create a pull request
	// consumes:
	// - application/json
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreatePullRequestOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/PullRequest"
	//   "409":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	var (
		repo        = ctx.Repo.Repository
		labelIDs    []int64
		assigneeID  int64
		milestoneID int64
	)

	// Get repo/branch information
	_, headRepo, headGitRepo, compareInfo, baseBranch, headBranch := parseCompareInfo(ctx, form)
	if ctx.Written() {
		return
	}
	defer headGitRepo.Close()

	// Check if another PR exists with the same targets
	existingPr, err := models.GetUnmergedPullRequest(headRepo.ID, ctx.Repo.Repository.ID, headBranch, baseBranch)
	if err != nil {
		if !models.IsErrPullRequestNotExist(err) {
			ctx.Error(http.StatusInternalServerError, "GetUnmergedPullRequest", err)
			return
		}
	} else {
		err = models.ErrPullRequestAlreadyExists{
			ID:         existingPr.ID,
			IssueID:    existingPr.Index,
			HeadRepoID: existingPr.HeadRepoID,
			BaseRepoID: existingPr.BaseRepoID,
			HeadBranch: existingPr.HeadBranch,
			BaseBranch: existingPr.BaseBranch,
		}
		ctx.Error(http.StatusConflict, "GetUnmergedPullRequest", err)
		return
	}

	if len(form.Labels) > 0 {
		labels, err := models.GetLabelsInRepoByIDs(ctx.Repo.Repository.ID, form.Labels)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetLabelsInRepoByIDs", err)
			return
		}

		labelIDs = make([]int64, len(form.Labels))
		orgLabelIDs := make([]int64, len(form.Labels))

		for i := range labels {
			labelIDs[i] = labels[i].ID
		}

		if ctx.Repo.Owner.IsOrganization() {
			orgLabels, err := models.GetLabelsInOrgByIDs(ctx.Repo.Owner.ID, form.Labels)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetLabelsInOrgByIDs", err)
				return
			}

			for i := range orgLabels {
				orgLabelIDs[i] = orgLabels[i].ID
			}
		}

		labelIDs = append(labelIDs, orgLabelIDs...)
	}

	if form.Milestone > 0 {
		milestone, err := models.GetMilestoneByRepoID(ctx.Repo.Repository.ID, milestoneID)
		if err != nil {
			if models.IsErrMilestoneNotExist(err) {
				ctx.NotFound()
			} else {
				ctx.Error(http.StatusInternalServerError, "GetMilestoneByRepoID", err)
			}
			return
		}

		milestoneID = milestone.ID
	}

	var deadlineUnix timeutil.TimeStamp
	if form.Deadline != nil {
		deadlineUnix = timeutil.TimeStamp(form.Deadline.Unix())
	}

	prIssue := &models.Issue{
		RepoID:       repo.ID,
		Title:        form.Title,
		PosterID:     ctx.User.ID,
		Poster:       ctx.User,
		MilestoneID:  milestoneID,
		AssigneeID:   assigneeID,
		IsPull:       true,
		Content:      form.Body,
		DeadlineUnix: deadlineUnix,
	}
	pr := &models.PullRequest{
		HeadRepoID: headRepo.ID,
		BaseRepoID: repo.ID,
		HeadBranch: headBranch,
		BaseBranch: baseBranch,
		HeadRepo:   headRepo,
		BaseRepo:   repo,
		MergeBase:  compareInfo.MergeBase,
		Type:       models.PullRequestGitea,
	}

	// Get all assignee IDs
	assigneeIDs, err := models.MakeIDsFromAPIAssigneesToAdd(form.Assignee, form.Assignees)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Assignee does not exist: [name: %s]", err))
		} else {
			ctx.Error(http.StatusInternalServerError, "AddAssigneeByName", err)
		}
		return
	}
	// Check if the passed assignees is assignable
	for _, aID := range assigneeIDs {
		assignee, err := models.GetUserByID(aID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserByID", err)
			return
		}

		valid, err := models.CanBeAssigned(assignee, repo, true)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "canBeAssigned", err)
			return
		}
		if !valid {
			ctx.Error(http.StatusUnprocessableEntity, "canBeAssigned", models.ErrUserDoesNotHaveAccessToRepo{UserID: aID, RepoName: repo.Name})
			return
		}
	}

	if err := pull_service.NewPullRequest(repo, prIssue, labelIDs, []string{}, pr, assigneeIDs); err != nil {
		if models.IsErrUserDoesNotHaveAccessToRepo(err) {
			ctx.Error(http.StatusBadRequest, "UserDoesNotHaveAccessToRepo", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "NewPullRequest", err)
		return
	}

	log.Trace("Pull request created: %d/%d", repo.ID, prIssue.ID)
	ctx.JSON(http.StatusCreated, convert.ToAPIPullRequest(pr))
}

// EditPullRequest does what it says
func EditPullRequest(ctx *context.APIContext, form api.EditPullRequestOption) {
	// swagger:operation PATCH /repos/{owner}/{repo}/pulls/{index} repository repoEditPullRequest
	// ---
	// summary: Update a pull request. If using deadline only the date will be taken into account, and time of day ignored.
	// consumes:
	// - application/json
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
	// - name: index
	//   in: path
	//   description: index of the pull request to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditPullRequestOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/PullRequest"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "409":
	//     "$ref": "#/responses/error"
	//   "412":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	err = pr.LoadIssue()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}
	issue := pr.Issue
	issue.Repo = ctx.Repo.Repository

	if !issue.IsPoster(ctx.User.ID) && !ctx.Repo.CanWrite(models.UnitTypePullRequests) {
		ctx.Status(http.StatusForbidden)
		return
	}

	oldTitle := issue.Title
	if len(form.Title) > 0 {
		issue.Title = form.Title
	}
	if len(form.Body) > 0 {
		issue.Content = form.Body
	}

	// Update or remove deadline if set
	if form.Deadline != nil || form.RemoveDeadline != nil {
		var deadlineUnix timeutil.TimeStamp
		if (form.RemoveDeadline == nil || !*form.RemoveDeadline) && !form.Deadline.IsZero() {
			deadline := time.Date(form.Deadline.Year(), form.Deadline.Month(), form.Deadline.Day(),
				23, 59, 59, 0, form.Deadline.Location())
			deadlineUnix = timeutil.TimeStamp(deadline.Unix())
		}

		if err := models.UpdateIssueDeadline(issue, deadlineUnix, ctx.User); err != nil {
			ctx.Error(http.StatusInternalServerError, "UpdateIssueDeadline", err)
			return
		}
		issue.DeadlineUnix = deadlineUnix
	}

	// Add/delete assignees

	// Deleting is done the GitHub way (quote from their api documentation):
	// https://developer.github.com/v3/issues/#edit-an-issue
	// "assignees" (array): Logins for Users to assign to this issue.
	// Pass one or more user logins to replace the set of assignees on this Issue.
	// Send an empty array ([]) to clear all assignees from the Issue.

	if ctx.Repo.CanWrite(models.UnitTypePullRequests) && (form.Assignees != nil || len(form.Assignee) > 0) {
		err = issue_service.UpdateAssignees(issue, form.Assignee, form.Assignees, ctx.User)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Assignee does not exist: [name: %s]", err))
			} else {
				ctx.Error(http.StatusInternalServerError, "UpdateAssignees", err)
			}
			return
		}
	}

	if ctx.Repo.CanWrite(models.UnitTypePullRequests) && form.Milestone != 0 &&
		issue.MilestoneID != form.Milestone {
		oldMilestoneID := issue.MilestoneID
		issue.MilestoneID = form.Milestone
		if err = issue_service.ChangeMilestoneAssign(issue, ctx.User, oldMilestoneID); err != nil {
			ctx.Error(http.StatusInternalServerError, "ChangeMilestoneAssign", err)
			return
		}
	}

	if ctx.Repo.CanWrite(models.UnitTypePullRequests) && form.Labels != nil {
		labels, err := models.GetLabelsInRepoByIDs(ctx.Repo.Repository.ID, form.Labels)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetLabelsInRepoByIDsError", err)
			return
		}

		if ctx.Repo.Owner.IsOrganization() {
			orgLabels, err := models.GetLabelsInOrgByIDs(ctx.Repo.Owner.ID, form.Labels)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetLabelsInOrgByIDs", err)
				return
			}

			labels = append(labels, orgLabels...)
		}

		if err = issue.ReplaceLabels(labels, ctx.User); err != nil {
			ctx.Error(http.StatusInternalServerError, "ReplaceLabelsError", err)
			return
		}
	}

	if form.State != nil {
		issue.IsClosed = (api.StateClosed == api.StateType(*form.State))
	}
	statusChangeComment, titleChanged, err := models.UpdateIssueByAPI(issue, ctx.User)
	if err != nil {
		if models.IsErrDependenciesLeft(err) {
			ctx.Error(http.StatusPreconditionFailed, "DependenciesLeft", "cannot close this pull request because it still has open dependencies")
			return
		}
		ctx.Error(http.StatusInternalServerError, "UpdateIssueByAPI", err)
		return
	}

	if titleChanged {
		notification.NotifyIssueChangeTitle(ctx.User, issue, oldTitle)
	}

	if statusChangeComment != nil {
		notification.NotifyIssueChangeStatus(ctx.User, issue, statusChangeComment, issue.IsClosed)
	}

	// change pull target branch
	if len(form.Base) != 0 && form.Base != pr.BaseBranch {
		if !ctx.Repo.GitRepo.IsBranchExist(form.Base) {
			ctx.Error(http.StatusNotFound, "NewBaseBranchNotExist", fmt.Errorf("new base '%s' not exist", form.Base))
			return
		}
		if err := pull_service.ChangeTargetBranch(pr, ctx.User, form.Base); err != nil {
			if models.IsErrPullRequestAlreadyExists(err) {
				ctx.Error(http.StatusConflict, "IsErrPullRequestAlreadyExists", err)
				return
			} else if models.IsErrIssueIsClosed(err) {
				ctx.Error(http.StatusUnprocessableEntity, "IsErrIssueIsClosed", err)
				return
			} else if models.IsErrPullRequestHasMerged(err) {
				ctx.Error(http.StatusConflict, "IsErrPullRequestHasMerged", err)
				return
			} else {
				ctx.InternalServerError(err)
			}
			return
		}
		notification.NotifyPullRequestChangeTargetBranch(ctx.User, pr, form.Base)
	}

	// Refetch from database
	pr, err = models.GetPullRequestByIndex(ctx.Repo.Repository.ID, pr.Index)
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	// TODO this should be 200, not 201
	ctx.JSON(http.StatusCreated, convert.ToAPIPullRequest(pr))
}

// IsPullRequestMerged checks if a PR exists given an index
func IsPullRequestMerged(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/merge repository repoPullRequestIsMerged
	// ---
	// summary: Check if a pull request has been merged
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
	// - name: index
	//   in: path
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     description: pull request has been merged
	//   "404":
	//     description: pull request has not been merged

	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if pr.HasMerged {
		ctx.Status(http.StatusNoContent)
	}
	ctx.NotFound()
}

// MergePullRequest merges a PR given an index
func MergePullRequest(ctx *context.APIContext, form auth.MergePullRequestForm) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/merge repository repoMergePullRequest
	// ---
	// summary: Merge a pull request
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
	// - name: index
	//   in: path
	//   description: index of the pull request to merge
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     $ref: "#/definitions/MergePullRequestOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/empty"
	//   "405":
	//     "$ref": "#/responses/empty"
	//   "409":
	//     "$ref": "#/responses/error"

	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if err = pr.LoadHeadRepo(); err != nil {
		ctx.ServerError("LoadHeadRepo", err)
		return
	}

	err = pr.LoadIssue()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}
	pr.Issue.Repo = ctx.Repo.Repository

	if ctx.IsSigned {
		// Update issue-user.
		if err = pr.Issue.ReadBy(ctx.User.ID); err != nil {
			ctx.Error(http.StatusInternalServerError, "ReadBy", err)
			return
		}
	}

	if pr.Issue.IsClosed {
		ctx.NotFound()
		return
	}

	allowedMerge, err := pull_service.IsUserAllowedToMerge(pr, ctx.Repo.Permission, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "IsUSerAllowedToMerge", err)
		return
	}
	if !allowedMerge {
		ctx.Error(http.StatusMethodNotAllowed, "Merge", "User not allowed to merge PR")
		return
	}

	if !pr.CanAutoMerge() || pr.HasMerged || pr.IsWorkInProgress() {
		ctx.Status(http.StatusMethodNotAllowed)
		return
	}

	if err := pull_service.CheckPRReadyToMerge(pr); err != nil {
		if !models.IsErrNotAllowedToMerge(err) {
			ctx.Error(http.StatusInternalServerError, "CheckPRReadyToMerge", err)
			return
		}
		if form.ForceMerge != nil && *form.ForceMerge {
			if isRepoAdmin, err := models.IsUserRepoAdmin(pr.BaseRepo, ctx.User); err != nil {
				ctx.Error(http.StatusInternalServerError, "IsUserRepoAdmin", err)
				return
			} else if !isRepoAdmin {
				ctx.Error(http.StatusMethodNotAllowed, "Merge", "Only repository admin can merge if not all checks are ok (force merge)")
			}
		} else {
			ctx.Error(http.StatusMethodNotAllowed, "PR is not ready to be merged", err)
			return
		}
	}

	if _, err := pull_service.IsSignedIfRequired(pr, ctx.User); err != nil {
		if !models.IsErrWontSign(err) {
			ctx.Error(http.StatusInternalServerError, "IsSignedIfRequired", err)
			return
		}
		ctx.Error(http.StatusMethodNotAllowed, fmt.Sprintf("Protected branch %s requires signed commits but this merge would not be signed", pr.BaseBranch), err)
		return
	}

	if len(form.Do) == 0 {
		form.Do = string(models.MergeStyleMerge)
	}

	message := strings.TrimSpace(form.MergeTitleField)
	if len(message) == 0 {
		if models.MergeStyle(form.Do) == models.MergeStyleMerge {
			message = pr.GetDefaultMergeMessage()
		}
		if models.MergeStyle(form.Do) == models.MergeStyleSquash {
			message = pr.GetDefaultSquashMessage()
		}
	}

	form.MergeMessageField = strings.TrimSpace(form.MergeMessageField)
	if len(form.MergeMessageField) > 0 {
		message += "\n\n" + form.MergeMessageField
	}

	if err := pull_service.Merge(pr, ctx.User, ctx.Repo.GitRepo, models.MergeStyle(form.Do), message); err != nil {
		if models.IsErrInvalidMergeStyle(err) {
			ctx.Status(http.StatusMethodNotAllowed)
			return
		} else if models.IsErrMergeConflicts(err) {
			conflictError := err.(models.ErrMergeConflicts)
			ctx.JSON(http.StatusConflict, conflictError)
		} else if models.IsErrRebaseConflicts(err) {
			conflictError := err.(models.ErrRebaseConflicts)
			ctx.JSON(http.StatusConflict, conflictError)
		} else if models.IsErrMergeUnrelatedHistories(err) {
			conflictError := err.(models.ErrMergeUnrelatedHistories)
			ctx.JSON(http.StatusConflict, conflictError)
		} else if git.IsErrPushOutOfDate(err) {
			ctx.Error(http.StatusConflict, "Merge", "merge push out of date")
			return
		} else if git.IsErrPushRejected(err) {
			errPushRej := err.(*git.ErrPushRejected)
			if len(errPushRej.Message) == 0 {
				ctx.Error(http.StatusConflict, "Merge", "PushRejected without remote error message")
				return
			}
			ctx.Error(http.StatusConflict, "Merge", "PushRejected with remote message: "+errPushRej.Message)
			return
		}
		ctx.Error(http.StatusInternalServerError, "Merge", err)
		return
	}

	log.Trace("Pull request merged: %d", pr.ID)
	ctx.Status(http.StatusOK)
}

// CompareAndPreMerged get a comparison of the two versions via shaes / branches / tags and return whether it can be merged
func CompareAndPreMerged(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/preMerged repository repoCompareAndPreMerged
	// ---
	// summary: Get a comparison of the two versions via shaes / branches / tags and return whether it can be merged
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
	// - name: shaes
	//   in: path
	//   description: The version to compare, use '...' to split
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of commits in the result to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of commits in the result, maximum page size is 50
	//   type: integer

	// responses:
	//   "200":
	//     "$ref": "#/responses/CompareAndPreMerged"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch, notMatterRepos, isSameRepo, headBranches, baseIs, headIs, hasErr := parsePrepareCompareDiffInfo(ctx)
	if hasErr {
		return
	}
	defer headGitRepo.Close()
	headCommitID, baseCommitID, diffRaw, nothingToCompare, diffNotAvailable, err := prepareCompareDiff(ctx, headUser, headRepo, headGitRepo, baseIs, headIs, compareInfo, baseBranch, headBranch)
	if err != nil {
		ctx.ServerError("prepareCompareDiff", err)
		return
	}

	compareAndPreMerged := new(api.CompareAndPreMerged)
	compareAndPreMerged.NotMatterRepos = notMatterRepos
	compareAndPreMerged.SameRepo = isSameRepo
	compareAndPreMerged.NothingToCompare = nothingToCompare
	compareAndPreMerged.DiffNotAvailable = diffNotAvailable
	compareAndPreMerged.BaseCommitID = baseCommitID
	compareAndPreMerged.HeadCommitID = headCommitID

	if !notMatterRepos {
		curHeadBranches, err := headGitRepo.GetBranches()
		if err != nil {
			ctx.ServerError("GetBranches", err)
			return
		}

		headBranches["head"] = curHeadBranches

		_, err = models.GetUnmergedPullRequest(headRepo.ID, ctx.Repo.Repository.ID, headBranch, baseBranch)
		if err != nil {
			if !models.IsErrPullRequestNotExist(err) {
				ctx.ServerError("GetUnmergedPullRequest", err)
				return
			}
		} else {
			compareAndPreMerged.HasPullRequest = true
			ctx.JSON(http.StatusOK, compareAndPreMerged)
			return
		}
	}

	compareAndPreMerged.DiffRaw = diffRaw
	compareAndPreMerged.HeadBranch = headBranch
	compareAndPreMerged.HeadBranches = headBranches
	compareAndPreMerged.Commits = make([]*api.Commit, 0, compareInfo.Commits.Len())
	for cur := compareInfo.Commits.Front(); cur != nil; cur = cur.Next() {
		// Convert git.Commit to api.Commit.
		commit, err := toCommit(ctx, ctx.Repo.Repository, cur.Value.(*git.Commit), nil)
		if err != nil {
			ctx.ServerError("CommitToApi", err)
			return
		}
		compareAndPreMerged.Commits = append(compareAndPreMerged.Commits, commit)
	}

	compareAndPreMerged.CommitCount = int64(compareInfo.Commits.Len())

	ctx.JSON(http.StatusOK, compareAndPreMerged)
}

func parseCompareInfo(ctx *context.APIContext, form api.CreatePullRequestOption) (*models.User, *models.Repository, *git.Repository, *git.CompareInfo, string, string) {
	baseRepo := ctx.Repo.Repository

	// Get compared branches information
	// format: <base branch>...[<head repo>:]<head branch>
	// base<-head: master...head:feature
	// same repo: master...feature

	// TODO: Validate form first?

	baseBranch := form.Base

	var (
		headUser   *models.User
		headBranch string
		isSameRepo bool
		err        error
	)

	// If there is no head repository, it means pull request between same repository.
	headInfos := strings.Split(form.Head, ":")
	if len(headInfos) == 1 {
		isSameRepo = true
		headUser = ctx.Repo.Owner
		headBranch = headInfos[0]

	} else if len(headInfos) == 2 {
		headUser, err = models.GetUserByName(headInfos[0])
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.NotFound("GetUserByName")
			} else {
				ctx.ServerError("GetUserByName", err)
			}
			return nil, nil, nil, nil, "", ""
		}
		headBranch = headInfos[1]

	} else {
		ctx.NotFound()
		return nil, nil, nil, nil, "", ""
	}

	ctx.Repo.PullRequest.SameRepo = isSameRepo
	log.Info("Base branch: %s", baseBranch)
	log.Info("Repo path: %s", ctx.Repo.GitRepo.Path)
	// Check if base branch is valid.
	if !ctx.Repo.GitRepo.IsBranchExist(baseBranch) {
		ctx.NotFound("IsBranchExist")
		return nil, nil, nil, nil, "", ""
	}

	// Check if current user has fork of repository or in the same repository.
	headRepo, has := models.HasForkedRepo(headUser.ID, baseRepo.ID)
	if !has && !isSameRepo {
		log.Trace("parseCompareInfo[%d]: does not have fork or in same repository", baseRepo.ID)
		ctx.NotFound("HasForkedRepo")
		return nil, nil, nil, nil, "", ""
	}

	var headGitRepo *git.Repository
	if isSameRepo {
		headRepo = ctx.Repo.Repository
		headGitRepo = ctx.Repo.GitRepo
	} else {
		headGitRepo, err = git.OpenRepository(models.RepoPath(headUser.Name, headRepo.Name))
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
			return nil, nil, nil, nil, "", ""
		}
	}

	// user should have permission to read baseRepo's codes and pulls, NOT headRepo's
	permBase, err := models.GetUserRepoPermission(baseRepo, ctx.User)
	if err != nil {
		headGitRepo.Close()
		ctx.ServerError("GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", ""
	}
	if !permBase.CanReadIssuesOrPulls(true) || !permBase.CanRead(models.UnitTypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User %-v cannot create/read pull requests or cannot read code in Repo %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.User,
				baseRepo,
				permBase)
		}
		headGitRepo.Close()
		ctx.NotFound("Can't read pulls or can't read UnitTypeCode")
		return nil, nil, nil, nil, "", ""
	}

	// user should have permission to read headrepo's codes
	permHead, err := models.GetUserRepoPermission(headRepo, ctx.User)
	if err != nil {
		headGitRepo.Close()
		ctx.ServerError("GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", ""
	}
	if !permHead.CanRead(models.UnitTypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in headRepo has Permissions: %-+v",
				ctx.User,
				headRepo,
				permHead)
		}
		headGitRepo.Close()
		ctx.NotFound("Can't read headRepo UnitTypeCode")
		return nil, nil, nil, nil, "", ""
	}

	// Check if head branch is valid.
	if !headGitRepo.IsBranchExist(headBranch) {
		headGitRepo.Close()
		ctx.NotFound()
		return nil, nil, nil, nil, "", ""
	}

	compareInfo, err := headGitRepo.GetCompareInfo(models.RepoPath(baseRepo.Owner.Name, baseRepo.Name), baseBranch, headBranch)
	if err != nil {
		headGitRepo.Close()
		ctx.Error(http.StatusInternalServerError, "GetCompareInfo", err)
		return nil, nil, nil, nil, "", ""
	}

	return headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch
}

func parsePrepareCompareDiffInfo(ctx *context.APIContext) (*models.User, *models.Repository, *git.Repository, *git.CompareInfo, string, string, bool, bool, map[string][]string, map[string]bool, map[string]bool, bool) {
	baseRepo := ctx.Repo.Repository

	// Get compared branches information
	// A full compare url is of the form:
	//
	// 1. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headBranch}
	// 2. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headOwner}:{:headBranch}
	// 3. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headOwner}/{:headRepoName}:{:headBranch}
	//
	// Here we obtain the infoPath "{:baseBranch}...[{:headOwner}/{:headRepoName}:]{:headBranch}" as ctx.Params("*")
	// with the :baseRepo in ctx.Repo.
	//
	// Note: Generally :headRepoName is not provided here - we are only passed :headOwner.
	//
	// How do we determine the :headRepo?
	//
	// 1. If :headOwner is not set then the :headRepo = :baseRepo
	// 2. If :headOwner is set - then look for the fork of :baseRepo owned by :headOwner
	// 3. But... :baseRepo could be a fork of :headOwner's repo - so check that
	// 4. Now, :baseRepo and :headRepos could be forks of the same repo - so check that
	//
	// format: <base branch>...[<head repo>:]<head branch>
	// base<-head: master...head:feature
	// same repo: master...feature

	var (
		headUser       *models.User
		headRepo       *models.Repository
		headBranch     string
		isSameRepo     bool
		notMatterRepos bool
		err            error

		baseIs       = make(map[string]bool, 3)
		headIs       = make(map[string]bool, 3)
		headBranches = make(map[string][]string, 3)
	)
	infoPath := ctx.Params("*")
	infos := strings.SplitN(infoPath, "...", 2)
	if len(infos) != 2 {
		log.Trace("ParseCompareInfo[%d]: not enough compared branches information %s", baseRepo.ID, infos)
		ctx.NotFound("CompareAndPullRequest", nil)
		return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
	}

	baseBranch := infos[0]

	// If there is no head repository, it means compare between same repository.
	headInfos := strings.Split(infos[1], ":")
	if len(headInfos) == 1 {
		isSameRepo = true
		headUser = ctx.Repo.Owner
		headBranch = headInfos[0]
	} else if len(headInfos) == 2 {
		headInfosSplit := strings.Split(headInfos[0], "/")
		if len(headInfosSplit) == 1 {
			headUser, err = models.GetUserByName(headInfos[0])
			if err != nil {
				if models.IsErrUserNotExist(err) {
					ctx.NotFound("GetUserByName", nil)
				} else {
					ctx.ServerError("GetUserByName", err)
				}
				return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
			}
			headBranch = headInfos[1]
			isSameRepo = headUser.ID == ctx.Repo.Owner.ID
			if isSameRepo {
				headRepo = baseRepo
			}
		} else {
			headRepo, err = models.GetRepositoryByOwnerAndName(headInfosSplit[0], headInfosSplit[1])
			if err != nil {
				if models.IsErrRepoNotExist(err) {
					ctx.NotFound("GetRepositoryByOwnerAndName", nil)
				} else {
					ctx.ServerError("GetRepositoryByOwnerAndName", err)
				}
				return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
			}
			if err := headRepo.GetOwner(); err != nil {
				if models.IsErrUserNotExist(err) {
					ctx.NotFound("GetUserByName", nil)
				} else {
					ctx.ServerError("GetUserByName", err)
				}
				return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
			}
			headBranch = headInfos[1]
			headUser = headRepo.Owner
			isSameRepo = headRepo.ID == ctx.Repo.Repository.ID
		}
	} else {
		ctx.NotFound("CompareAndPullRequest", nil)
		return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
	}

	ctx.Repo.PullRequest.SameRepo = isSameRepo

	// Check if base branch is valid.
	baseIsCommit := ctx.Repo.GitRepo.IsCommitExist(baseBranch)
	baseIsBranch := ctx.Repo.GitRepo.IsBranchExist(baseBranch)
	baseIsTag := ctx.Repo.GitRepo.IsTagExist(baseBranch)
	if !baseIsCommit && !baseIsBranch && !baseIsTag {
		// Check if baseBranch is short sha commit hash
		if baseCommit, _ := ctx.Repo.GitRepo.GetCommit(baseBranch); baseCommit != nil {
			baseBranch = baseCommit.ID.String()
			baseIsCommit = true
		} else {
			ctx.NotFound("IsRefExist", nil)
			return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
		}
	}

	baseIs["branch"] = baseIsBranch
	baseIs["tag"] = baseIsTag
	baseIs["commit"] = baseIsCommit

	// Now we have the repository that represents the base

	// The current base and head repositories and branches may not
	// actually be the intended branches that the user wants to
	// create a pull-request from - but also determining the head
	// repo is difficult.

	// We will want therefore to offer a few repositories to set as
	// our base and head

	// 1. First if the baseRepo is a fork get the "RootRepo" it was
	// forked from
	var rootRepo *models.Repository
	if baseRepo.IsFork {
		err = baseRepo.GetBaseRepo()
		if err != nil {
			if !models.IsErrRepoNotExist(err) {
				ctx.ServerError("Unable to find root repo", err)
				return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
			}
		} else {
			rootRepo = baseRepo.BaseRepo
		}
	}

	// 2. Now if the current user is not the owner of the baseRepo,
	// check if they have a fork of the base repo and offer that as
	// "OwnForkRepo"
	var ownForkRepo *models.Repository
	if ctx.User != nil && baseRepo.OwnerID != ctx.User.ID {
		repo, has := models.HasForkedRepo(ctx.User.ID, baseRepo.ID)
		if has {
			ownForkRepo = repo
		}
	}

	has := headRepo != nil

	// 3. If the base is a forked from "RootRepo" and the owner of
	// the "RootRepo" is the :headUser - set headRepo to that
	if !has && rootRepo != nil && rootRepo.OwnerID == headUser.ID {
		headRepo = rootRepo
		has = true
	}

	// 4. If the ctx.User has their own fork of the baseRepo and the headUser is the ctx.User
	// set the headRepo to the ownFork
	if !has && ownForkRepo != nil && ownForkRepo.OwnerID == headUser.ID {
		headRepo = ownForkRepo
		has = true
	}

	// 5. If the headOwner has a fork of the baseRepo - use that
	if !has {
		headRepo, has = models.HasForkedRepo(headUser.ID, baseRepo.ID)
	}

	// 6. If the baseRepo is a fork and the headUser has a fork of that use that
	if !has && baseRepo.IsFork {
		headRepo, has = models.HasForkedRepo(headUser.ID, baseRepo.ForkID)
	}

	// 7. Otherwise if we're not the same repo and haven't found a repo give up
	if !isSameRepo && !has {
		notMatterRepos = true
	}

	// 8. Finally open the git repo
	var headGitRepo *git.Repository
	if isSameRepo {
		headRepo = ctx.Repo.Repository
		headGitRepo = ctx.Repo.GitRepo
	} else if has {
		headGitRepo, err = git.OpenRepository(headRepo.RepoPath())
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
		}
		defer headGitRepo.Close()
	}

	// Now we need to assert that the ctx.User has permission to read
	// the baseRepo's code and pulls
	// (NOT headRepo's)
	permBase, err := models.GetUserRepoPermission(baseRepo, ctx.User)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
	}
	if !permBase.CanRead(models.UnitTypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.User,
				baseRepo,
				permBase)
		}
		ctx.NotFound("ParseCompareInfo", nil)
		return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
	}

	// If we're not merging from the same repo:
	if !isSameRepo {
		// Assert ctx.User has permission to read headRepo's codes
		permHead, err := models.GetUserRepoPermission(headRepo, ctx.User)
		if err != nil {
			ctx.ServerError("GetUserRepoPermission", err)
			return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
		}
		if !permHead.CanRead(models.UnitTypeCode) {
			if log.IsTrace() {
				log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in headRepo has Permissions: %-+v",
					ctx.User,
					headRepo,
					permHead)
			}
			ctx.NotFound("ParseCompareInfo", nil)
			return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
		}
	}

	// If we have a rootRepo and it's different from:
	// 1. the computed base
	// 2. the computed head
	// then get the branches of it
	if rootRepo != nil &&
		rootRepo.ID != headRepo.ID &&
		rootRepo.ID != baseRepo.ID {
		perm, branches, err := getBranchesForRepo(ctx.User, rootRepo)
		if err != nil {
			ctx.ServerError("GetBranchesForRepo", err)
			return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
		}
		if perm {
			headBranches[rootRepo.FullName()] = branches
		}
	}

	// If we have a ownForkRepo and it's different from:
	// 1. The computed base
	// 2. The computed hea
	// 3. The rootRepo (if we have one)
	// then get the branches from it.
	if ownForkRepo != nil &&
		ownForkRepo.ID != headRepo.ID &&
		ownForkRepo.ID != baseRepo.ID &&
		(rootRepo == nil || ownForkRepo.ID != rootRepo.ID) {
		perm, branches, err := getBranchesForRepo(ctx.User, ownForkRepo)
		if err != nil {
			ctx.ServerError("GetBranchesForRepo", err)
			return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
		}
		if perm {
			headBranches[ownForkRepo.FullName()] = branches
		}
	}

	// Check if head branch is valid.
	headIsCommit := headGitRepo.IsCommitExist(headBranch)
	headIsBranch := headGitRepo.IsBranchExist(headBranch)
	headIsTag := headGitRepo.IsTagExist(headBranch)
	if !headIsCommit && !headIsBranch && !headIsTag {
		// Check if headBranch is short sha commit hash
		if headCommit, _ := headGitRepo.GetCommit(headBranch); headCommit != nil {
			headBranch = headCommit.ID.String()
			headIsCommit = true
		} else {
			ctx.NotFound("IsRefExist", nil)
			return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
		}
	}

	if !notMatterRepos && !permBase.CanReadIssuesOrPulls(true) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot create/read pull requests in Repo: %-v\nUser inbaseRepo has Permissions: %-+v",
				ctx.User,
				baseRepo,
				permBase)
		}
		ctx.NotFound("ParseCompareInfo", nil)
		return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
	}

	baseBranchRef := baseBranch
	if baseIsBranch {
		baseBranchRef = git.BranchPrefix + baseBranch
	} else if baseIsTag {
		baseBranchRef = git.TagPrefix + baseBranch
	}
	headIs["branch"] = headIsBranch
	headIs["tag"] = headIsTag
	headIs["commit"] = headIsCommit

	headBranchRef := headBranch
	if headIsBranch {
		headBranchRef = git.BranchPrefix + headBranch
	} else if headIsTag {
		headBranchRef = git.TagPrefix + headBranch
	}

	listOptions := utils.GetListOptions(ctx)
	if listOptions.Page <= 0 {
		listOptions.Page = 1
	}

	if listOptions.PageSize > git.CommitsRangeSize {
		listOptions.PageSize = git.CommitsRangeSize
	}

	compareInfo, err := headGitRepo.GetCompareInfo(baseRepo.RepoPath(), baseBranchRef, headBranchRef, listOptions.Page, listOptions.PageSize)
	if err != nil {
		ctx.ServerError("GetCompareInfo", err)
		return nil, nil, nil, nil, "", "", false, false, nil, nil, nil, true
	}

	return headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch, notMatterRepos, isSameRepo, headBranches, baseIs, headIs, false

}

func getBranchesForRepo(user *models.User, repo *models.Repository) (bool, []string, error) {
	perm, err := models.GetUserRepoPermission(repo, user)
	if err != nil {
		return false, nil, err
	}
	if !perm.CanRead(models.UnitTypeCode) {
		return false, nil, nil
	}
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return false, nil, err
	}
	defer gitRepo.Close()

	branches, err := gitRepo.GetBranches()
	if err != nil {
		return false, nil, err
	}
	return true, branches, nil
}

func prepareCompareDiff(
	ctx *context.APIContext,
	headUser *models.User,
	headRepo *models.Repository,
	headGitRepo *git.Repository,
	baseIs, headIs map[string]bool,
	compareInfo *git.CompareInfo,
	baseBranch, headBranch string) (string, string, string, bool, bool, error) {

	var err error

	headCommitID := headBranch
	if headIs["commit"] == false {
		if headIs["tag"] == true {
			headCommitID, err = headGitRepo.GetTagCommitID(headBranch)
		} else {
			headCommitID, err = headGitRepo.GetBranchCommitID(headBranch)
		}
		if err != nil {
			return "", "", "", false, false, fmt.Errorf("GetRefCommitID: %v", err)
		}
	}

	if headCommitID == compareInfo.MergeBase {
		return "", "", "", true, false, nil
	}

	baseGitRepo := ctx.Repo.GitRepo
	baseCommitID := baseBranch
	if baseIs["commit"] == false {
		if baseIs["tag"] == true {
			baseCommitID, err = baseGitRepo.GetTagCommitID(baseBranch)
		} else {
			baseCommitID, err = baseGitRepo.GetBranchCommitID(baseBranch)
		}
		if err != nil {
			return "", "", "", false, false, fmt.Errorf("GetRefCommitID: %v", err)
		}
	}

	// Get diff raw.
	rawBuffer := &bytes.Buffer{}
	var diffRaw string
	if err := git.GetRawDiffForFile(models.RepoPath(headUser.Name, headRepo.Name), baseCommitID, headCommitID, "diff", "", rawBuffer); err != nil {
		return "", "", "", false, false, fmt.Errorf("GetRawDiff: %v", err)
	}

	if len(rawBuffer.Bytes()) > MAX_DIFF_LEN {
		diffRaw = string(rawBuffer.Bytes()[:MAX_DIFF_LEN])
	} else {
		diffRaw = rawBuffer.String()
	}

	diffNotAvailable := len(diffRaw) == 0

	return headCommitID, baseCommitID, diffRaw, false, diffNotAvailable, nil
}
