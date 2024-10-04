package resources

import (
	"context"
	"fmt"
	"github.com/google/go-github/v47/github"
	"golang.org/x/oauth2"
)

// GithubClient is a wrapper for the GitHub client
type GithubClient struct {
	client *github.Client
}

// NewGithubClient initializes a new GitHub client using OAuth2
func NewGithubClient(token string) *GithubClient {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	return &GithubClient{client: client}
}

// CheckIssueExists checks if and issue with the same title exists in the repository
func (g *GithubClient) CheckIssueExists(owner, repo, title string) (*github.Issue, error) {
	issues, _, err := g.client.Issues.ListByRepo(context.Background(), owner, repo, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %w %s", err, owner)
	}

	// look for issue matching the title
	for _, issue := range issues {
		if issue.GetTitle() == title {
			return issue, nil
		}
	}

	return nil, nil
}

// CreateIssue creates a new GitHub issue in the specified repo
func (g *GithubClient) CreateIssue(owner, repo, title, description string) (*github.Issue, error) {
	newIssue := &github.IssueRequest{
		Title: &title,
		Body:  &description,
	}
	createdIssue, _, err := g.client.Issues.Create(context.Background(), owner, repo, newIssue)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return createdIssue, nil
}

func (g *GithubClient) UpdateIssue(owner, repo string, issue *github.Issue, description string) (*github.Issue, error) {
	// prepare an issue request for updating
	issueRequest := &github.IssueRequest{
		Body: &description, // for now only modify the issue description
	}

	updatedIssue, _, err := g.client.Issues.Edit(context.Background(), owner, repo, *issue.Number, issueRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to update issue: %w", err)
	}

	return updatedIssue, nil
}
