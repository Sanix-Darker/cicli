package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

const (
	baseURL = "https://api.github.com"
)

// GitHubToken should be set to your personal access token
const GitHubToken = os.getenv("GITHUB_TOKEN")

// will be filled later with extractRepoInfoFromGit (that takes from the current repo)
const (
	GitHubRepoOwner string
	GitHubRepoName  string
)

type logLine struct {
	Log string `json:"message"`
}

type Query struct {
	Repository struct {
		WorkflowRuns struct {
			Nodes []struct {
				ID int64
			}
		} `graphql:"workflow_runs(first: 1, event: $event, branch: $branch, pullRequest: $pullRequest)"`
	} `graphql:"repository(owner: $repositoryOwner, name: $repositoryName)"`
}
func extractRepoInfoFromGit() {
	repo, err := git.PlainOpen(".git")
	if err != nil {
		log.Fatal("Error opening the Git repository:", err)
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		log.Fatal("Error getting the remote 'origin':", err)
	}

	url := remote.Config().URLs[0]
	urlParts := strings.Split(url, "/")

	if len(urlParts) < 2 {
		log.Fatal("Invalid Git repository URL")
	}

	GitHubRepoOwner = urlParts[len(urlParts)-2]
	GitHubRepoName = strings.TrimSuffix(filepath.Base(urlParts[len(urlParts)-1]), ".git")
}
func getWorkflowRunID(branch, pullRequest string) (int64, error) {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: GitHubToken},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	client := githubv4.NewClient(httpClient)

	variables := map[string]interface{}{
		"repositoryOwner": githubv4.String(GitHubRepoOwner),
		"repositoryName":  githubv4.String(GitHubRepoName),
		"event":           githubv4.String("push"),
		"branch":          githubv4.String(branch),
		"pullRequest":     githubv4.String(pullRequest),
	}

	var query Query
	err := client.Query(context.Background(), &query, variables)
	if err != nil {
		return 0, err
	}

	if len(query.Repository.WorkflowRuns.Nodes) == 0 {
		return 0, fmt.Errorf("no workflow runs found")
	}

	return query.Repository.WorkflowRuns.Nodes[0].ID, nil
}

func followLogs(runID int64) error {
	req, err := http.NewRequest(
        "GET",
        fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/logs", baseURL, GitHubRepoOwner, GitHubRepoName, runID),
        nil
    )
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", GitHubToken))

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch logs, status code: %d", resp.StatusCode)
	}

	fmt.Println("Fetching GitHub Actions logs...")
	decoder := json.NewDecoder(resp.Body)

	for {
		var line logLine
		if err := decoder.Decode(&line); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		fmt.Println(line.Log)
	}

	return nil
}

func main() {
    // since we should be inside a .git repo
    extractRepoInfoFromGit()

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go BRANCH_NAME or go run main.go PULL_REQUEST_NUMBER")
		os.Exit(1)
	}

	input := os.Args[1]
	runID, err := getWorkflowRunID(input, input)
	if err != nil {
		log.Fatal("Error:", err)
	}

	if err := followLogs(runID); err != nil {
		log.Fatal("Error:", err)
	}
}
