package status

import (
	"context"
	"fmt"
	"github.com/google/go-github/v47/github"
	batchv1 "github.com/oshribelay/github-issue-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	log.FromContext(ctx).Info("Updating status...")

	// update the status of the GithubIssue CR
	if err := c.Status().Update(ctx, githubIssue); err != nil {
		return fmt.Errorf("failed to update GithubIssue status: %w", err)
	}

	return nil
}
