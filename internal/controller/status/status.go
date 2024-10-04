package status

import (
	"context"
	"fmt"
	"github.com/google/go-github/v47/github"
	batchv1 "github.com/oshribelay/github-issue-operator/api/v1"
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
			Reason:             "Issue is open",
		})
	} else {
		conditions = append(conditions, metav1.Condition{
			Type:               "IssueOpen",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "Issue is closed",
		})
	}

	// check if the issue has an associated PR
	if issue.PullRequestLinks != nil {
		conditions = append(conditions, metav1.Condition{
			Type:               "HasPR",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Issue has a pull request",
		})
	} else {
		conditions = append(conditions, metav1.Condition{
			Type:               "HasPR",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "Issue has no pull request",
		})
	}

	// update the status of the GithubIssue CR
	if err := c.Status().Update(ctx, githubIssue); err != nil {
		return fmt.Errorf("failed to update GithubIssue status: %w", err)
	}

	return nil
}
