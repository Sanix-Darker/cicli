package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	githubv4 "github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

const (
	baseURL = "https://api.github.com"
)

// GitHubToken should be set to your personal access token
var GitHubToken = os.Getenv("GITHUB_TOKEN")

// will be filled later with extractRepoInfoFromGit (that takes from the current repo)
var (
	GitHubRepoOwner string
	GitHubRepoName  string
)

type logLine struct {
	Log string `json:"message"`
}

type Query struct {
	Tree       struct{ CommitUrl string }
	Repository struct {
		Name string
		Ref  struct {
			BranchProtectionRule struct {
				RequiredApprovingReviewCount int
				RequiresApprovingReviews     bool
				RequiresCodeOwnerReviews     bool
				RequiresCommitSignatures     bool
			}
		} `graphql:"ref(qualifiedName: $branch)"`
	} `graphql:"repository(owner: $repositoryOwner, name: $repositoryName)"`
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
		"branch":          githubv4.String(branch),
	}

	var query Query
	err := client.Query(context.Background(), &query, variables)
	if err != nil {
		return 0, err
	}

	fmt.Printf(">>>><< %+v\n", query)
	if query.Repository.Name == "" {
		return 0, fmt.Errorf("branch '%s' not found in the repository", branch)
	}

	// Use the commit URL or any other information as needed
	fmt.Println("Commit URL:", query.Tree.CommitUrl)

	// You can add more conditions here based on the branch protection rule, pull request status, etc.
	// For example, you can check for pull request and extract more information using query.Repository.Ref.PullRequest

	// In this example, I am simply using the branch name as the workflow run ID
	runID := int64(len(query.Repository.Name))
	return runID, nil
}

func followLogs(runID int64) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/logs", baseURL, GitHubRepoOwner, GitHubRepoName, runID), nil)
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
func extractRepoInfoFromGit() {
	executablePath, err := os.Executable()
	if err != nil {
		log.Fatal("Error getting executable path:", err)
	}

	realPath, err := filepath.EvalSymlinks(executablePath)
	if err != nil {
		log.Fatal("Error resolving symlinks:", err)
	}

	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = filepath.Dir(realPath)
	output, err := cmd.Output()
	if err != nil {
		log.Fatal("Error getting remote 'origin':", err)
	}

	url := strings.TrimSpace(string(output))

	// Convert SSH URL to HTTPS URL
	if strings.HasPrefix(url, "git@github.com:") {
		urlParts := strings.Split(url, ":")
		if len(urlParts) == 2 {
			GitHubRepoOwner = strings.Split(urlParts[1], "/")[0]
			GitHubRepoName = strings.TrimSuffix(strings.Split(urlParts[1], "/")[1], ".git")
			fmt.Println("> Owner: ", GitHubRepoOwner)
			fmt.Println("> Project: ", GitHubRepoName)
		} else {
			log.Fatal("Invalid Git repository URL")
		}
	} else {
		// Handle HTTPS URL format if needed
	}

	if GitHubRepoOwner == "" || GitHubRepoName == "" {
		log.Fatal("Failed to extract repository owner and name from Git URL.")
	}
}

func main() {
	// since we should be inside a .git repo
	extractRepoInfoFromGit()

	fmt.Printf("> GitHub Repo : github.com/%s/%s\n", GitHubRepoOwner, GitHubRepoName)

	if len(os.Args) < 2 {
		fmt.Println("Usage: BRANCH_NAME=your-branch cicli or PULL_REQUEST_NUMBER=your-pr-number cicli")
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
