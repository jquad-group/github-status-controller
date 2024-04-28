package git

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v61/github"
)

type GithubClient struct {
	BaseUrl            string
	RepoOwner          string
	RepoName           string
	Revision           string
	AccessToken        string
	InsecureSkipVerify bool
}

func NewGithubClient(baseUrl string, repoOwner string, repoName string, revision string, accessToken string, insecureSkipVerify bool) *GithubClient {
	return &GithubClient{
		BaseUrl:            baseUrl,
		RepoOwner:          repoOwner,
		RepoName:           repoName,
		Revision:           revision,
		AccessToken:        accessToken,
		InsecureSkipVerify: insecureSkipVerify,
	}
}

func NewGithubAppClient(
	baseUrl string,
	repoOwner string,
	repoName string,
	revision string,
	appId int64,
	installationId int64,
	privateKeyFile string,
	insecureSkipVerify bool,
) (*GithubClient, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify,
		},
	}

	itr, err := ghinstallation.NewKeyFromFile(tr, appId, installationId, privateKeyFile)
	if err != nil {
		return nil, err
	}

	client, err := createClient(baseUrl, &http.Client{Transport: itr})
	if err != nil {
		return nil, err
	}

	token, _, err := client.Apps.CreateInstallationToken(
		context.Background(),
		installationId,
		&github.InstallationTokenOptions{})
	if err != nil {
		return nil, err
	}

	return &GithubClient{
		BaseUrl:            baseUrl,
		RepoOwner:          repoOwner,
		RepoName:           repoName,
		Revision:           revision,
		AccessToken:        token.GetToken(),
		InsecureSkipVerify: insecureSkipVerify,
	}, nil
}

func (githubClient *GithubClient) transport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: githubClient.InsecureSkipVerify,
		},
	}
}

func (githubClient *GithubClient) createClientWithAuthToken() (*github.Client, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: githubClient.InsecureSkipVerify,
			},
		},
	}

	client, err := createClient(githubClient.BaseUrl, httpClient)
	if err != nil {
		return nil, err
	}

	return client.WithAuthToken(githubClient.AccessToken), nil
}

func createClient(baseUrl string, httpClient *http.Client) (*github.Client, error) {
	parsedUrl, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	if parsedUrl.Host == "github.com" {
		return github.NewClient(httpClient), nil
	}

	return github.NewClient(httpClient).
		WithEnterpriseURLs(baseUrl, baseUrl)
}
