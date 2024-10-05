package status

import (
	"context"
	"fmt"
	"github.com/google/go-github/v47/github"
	batchv1 "github.com/oshribelay/github-issue-operator/api/v1"
	"github.com/oshribelay/github-issue-operator/internal/controller/resources"
	"github.com/oshribelay/github-issue-operator/internal/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Update(ctx context.Context, c client.Client, githubIssue *batchv1.GithubIssue, issue *github.Issue) error {
	conditions := []metav1.Condition{}

	// check if the issue is open
	if *issue.State == "open" {
		conditions = append(conditions, metav1.Condition{
			Type:               "IssueOpen",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "IssueIsOpen",
			Message:            fmt.Sprintf("Issue #%d is currently open", *issue.Number),
		})
	} else {
		conditions = append(conditions, metav1.Condition{
			Type:               "IssueOpen",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "IssueIsClosed",
			Message:            fmt.Sprintf("Issue #%d is closed", *issue.Number),
		})
	}

	// check if the issue has an associated PR
	if issue.PullRequestLinks != nil {
		conditions = append(conditions, metav1.Condition{
			Type:               "HasPR",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "PullRequestExists",
			Message:            "This issue has an associated pull request",
		})
	} else {
		conditions = append(conditions, metav1.Condition{
			Type:               "HasPR",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "NoPullRequest",
			Message:            "This issue does not have an associated pull request",
		})
	}

	// set the status fields to be updated
	githubIssue.Status.Conditions = conditions
	githubIssue.Status.IssueNumber = int32(*issue.Number)
	githubIssue.Status.LastUpdated = metav1.Now()

	// update the status of the GithubIssue CR
	if err := c.Status().Update(ctx, githubIssue); err != nil {
		return fmt.Errorf("failed to update GithubIssue status: %w", err)
	}

	return nil
}

func Delete(ctx context.Context, c client.Client, gClient *resources.GithubClient, githubIssue *batchv1.GithubIssue) error {
	owner, repo, err := utils.ParseRepoUrl(githubIssue.Spec.Repo)
	issueNumber := int(githubIssue.Status.IssueNumber)
	if err != nil {
		return fmt.Errorf("failed to parse repo url: %w", err)
	}

	// check if the issue exists
	issue, err := gClient.CheckIssueExists(owner, repo, githubIssue.Spec.Title, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to check if issue exists: %w", err)
	}

	// close the issue if it exists and still open
	if issue != nil && *issue.State == "open" {
		err := gClient.CloseIssue(owner, repo, issue)
		if err != nil {
			return fmt.Errorf("failed to close issue: %w", err)
		}
	}

	// remove the GithubIssue CR from the cluster
	if err := c.Delete(ctx, githubIssue); err != nil {
		return fmt.Errorf("failed to delete GithubIssue: %w", err)
	}

	return nil
}
