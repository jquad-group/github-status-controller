package git

import (
	"context"

	"github.com/google/go-github/v61/github"
)

func (githubClient *GithubClient) SetStatus(ctx context.Context, ghState, ghDescription, ghContext, ghTargetUrl string) (error, bool) {
	client, err := githubClient.createClientWithAuthToken()
	if err != nil {
		return err, false
	}

	// create the status
	status := &github.RepoStatus{
		State:       github.String(ghState),
		Description: github.String(ghDescription),
		Context:     github.String(ghContext),
		TargetURL:   github.String(ghTargetUrl),
	}

	_, _, err = client.Repositories.CreateStatus(ctx, githubClient.RepoOwner, githubClient.RepoName, githubClient.Revision, status)
	if err != nil {
		return err, false
	}

	return nil, true
}
