package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	githubAPIURL = "https://api.github.com/graphql"
	username     = "wangke19"
)

type GitHubStats struct {
	TotalStars         int
	TotalCommits       int
	TotalPRs           int
	TotalIssues        int
	ContributedTo      int
	PublicRepos        int
	TotalContributions int
}

func main() {
	ctx := context.Background()

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Fprintf(os.Stderr, "GITHUB_TOKEN is required\n")
		os.Exit(1)
	}

	stats, err := fetchGitHubStats(ctx, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch stats: %v\n", err)
		os.Exit(1)
	}

	if err := updateREADME(stats); err != nil {
		fmt.Fprintf(os.Stderr, "failed to update README: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("README updated successfully")
}

func fetchGitHubStats(ctx context.Context, token string) (*GitHubStats, error) {
	query := `{
		user(login: "` + username + `") {
			contributionsCollection {
				totalCommitContributions
				totalIssueContributions
				totalPullRequestContributions
				contributionCalendar {
					totalContributions
				}
			}
			repositoriesContributedTo(first: 1, contributionTypes: [COMMIT, ISSUE, PULL_REQUEST]) {
				totalCount
			}
			repositories(first: 100, ownerAffiliations: OWNER) {
				totalCount
				nodes {
					stargazerCount
				}
			}
		}
	}`

	reqBody := map[string]string{"query": query}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", githubAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Data struct {
			User struct {
				ContributionsCollection struct {
					TotalCommitContributions      int `json:"totalCommitContributions"`
					TotalIssueContributions       int `json:"totalIssueContributions"`
					TotalPullRequestContributions int `json:"totalPullRequestContributions"`
					ContributionCalendar          struct {
						TotalContributions int `json:"totalContributions"`
					} `json:"contributionCalendar"`
				} `json:"contributionsCollection"`
				RepositoriesContributedTo struct {
					TotalCount int `json:"totalCount"`
				} `json:"repositoriesContributedTo"`
				Repositories struct {
					TotalCount int `json:"totalCount"`
					Nodes      []struct {
						StargazerCount int `json:"stargazerCount"`
					} `json:"nodes"`
				} `json:"repositories"`
			} `json:"user"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	totalStars := 0
	for _, repo := range result.Data.User.Repositories.Nodes {
		totalStars += repo.StargazerCount
	}

	return &GitHubStats{
		TotalStars:         totalStars,
		TotalCommits:       result.Data.User.ContributionsCollection.TotalCommitContributions,
		TotalPRs:           result.Data.User.ContributionsCollection.TotalPullRequestContributions,
		TotalIssues:        result.Data.User.ContributionsCollection.TotalIssueContributions,
		ContributedTo:      result.Data.User.RepositoriesContributedTo.TotalCount,
		PublicRepos:        result.Data.User.Repositories.TotalCount,
		TotalContributions: result.Data.User.ContributionsCollection.ContributionCalendar.TotalContributions,
	}, nil
}

func updateREADME(stats *GitHubStats) error {
	content, err := os.ReadFile("README.md")
	if err != nil {
		return fmt.Errorf("read README: %w", err)
	}

	statsSection := fmt.Sprintf(`## 📊 GitHub Stats

`+"```"+`text
💫 Total Stars                %d
🔥 Total Commits              %d
🔀 Total PRs                  %d
📝 Total Issues               %d
🤝 Contributed to             %d repositories
📦 Public Repositories        %d

Last updated: %s
`+"```"+`

---`,
		stats.TotalStars,
		stats.TotalCommits,
		stats.TotalPRs,
		stats.TotalIssues,
		stats.ContributedTo,
		stats.PublicRepos,
		time.Now().UTC().Format("2006-01-02"),
	)

	text := string(content)
	start := strings.Index(text, "## 📊 GitHub Stats")
	if start == -1 {
		return fmt.Errorf("stats section not found in README")
	}

	end := strings.Index(text[start:], "\n---")
	if end == -1 {
		return fmt.Errorf("end of stats section not found in README")
	}
	end += start + len("\n---")

	newContent := text[:start] + statsSection + text[end:]

	if err := os.WriteFile("README.md", []byte(newContent), 0644); err != nil {
		return fmt.Errorf("write README: %w", err)
	}

	return nil
}
