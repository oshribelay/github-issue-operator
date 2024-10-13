package finalizer

import (
	"context"
	v1 "github.com/oshribelay/github-issue-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const githubIssueFinalizer = "finalizer.githubissue.issue.core.github.io"

func EnsureFinalizer(ctx context.Context, c client.Client, githubIssue *v1.GithubIssue) error {
	if !controllerutil.ContainsFinalizer(githubIssue, githubIssueFinalizer) {
		controllerutil.AddFinalizer(githubIssue, githubIssueFinalizer)
		if err := c.Update(ctx, githubIssue); err != nil {
			return err
		}
	}
	return nil
}

func RemoveFinalizer(ctx context.Context, c client.Client, githubIssue *v1.GithubIssue) error {
	if controllerutil.ContainsFinalizer(githubIssue, githubIssueFinalizer) {
		controllerutil.RemoveFinalizer(githubIssue, githubIssueFinalizer)
		if err := c.Update(ctx, githubIssue); err != nil {
			return err
		}
	}
	return nil
}
