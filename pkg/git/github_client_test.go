package git

import (
	"context"
	"os"
	"testing"
)

func TestSimpleAuth(t *testing.T) {
	at, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok {
		t.Fatal("GITHUB_TOKEN not set")
	}

	c := NewGithubClient("https://github.com", "owner", "repo", "main", at, false)

	client, err := c.createClientWithAuthToken()
	if err != nil {
		t.Fatal(err)
	}

	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}

	if user == nil {
		t.Fatal("user is nil")
	}
}

func TestAppAuth(t *testing.T) {
	// TODO: look up github docs how to test this
	// appId int64,
	// installationId int64,
	// privateKeyFile string,
	// c, err := NewGithubAppClient("https://github.com", "owner", "repo", "main", 0, 0, "", false)
	// if err != nil {
	// 	t.Fatal(err)
	// }
}
