package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"os/exec"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Repository は表示するリポジトリの情報を保持する構造体
type Repository struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Stars       int    `json:"stargazers_count"`
	OrgName     string
	HtmlUrl     string `json:"html_url"`
	Host        string
	Cloned      bool
}

// Define a common Organization type
type Organization struct {
	Login string `json:"login"`
}

// pecoで選択するための文字列を生成する関数
func formatRepoLine(repo Repository) string {
	cloneStatus := " "
	if repo.Cloned {
		cloneStatus = "✓"
	}
	return fmt.Sprintf("%s %s/%s/%s", cloneStatus, repo.Host, repo.OrgName, repo.Name)
}

// リポジトリに関連するメソッドを追加
func (r Repository) GetClonePath() string {
	return filepath.Join(os.Getenv("HOME"), "ghq", r.Host, r.OrgName, r.Name)
}

func (r Repository) GetGitURL() string {
	return fmt.Sprintf("git@%s:%s/%s", r.Host, r.OrgName, r.Name)
}

// エラーハンドリング用のヘルパー関数
func handleError(err error, message string) {
	if err != nil {
		fmt.Printf("%s: %v\n", message, err)
		os.Exit(1)
	}
}

// Update fetchOrganizations to use the new type
func fetchOrganizations(client *api.RESTClient) []Organization {
	var orgs []Organization
	err := client.Get("user/orgs", &orgs)
	handleError(err, "組織の取得に失敗")
	return orgs
}

// Update fetchRepositories parameter type
func fetchRepositories(client *api.RESTClient, orgs []Organization) []Repository {
	var allRepos []Repository
	for _, org := range orgs {
		var repos []Repository
		err := client.Get(fmt.Sprintf("orgs/%s/repos", org.Login), &repos)
		if err != nil {
			fmt.Printf("リポジトリの取得に失敗 (%s): %v\n", org.Login, err)
			continue
		}
		for i := range repos {
			repos[i].OrgName = org.Login
			hostWithPath := strings.TrimPrefix(repos[i].HtmlUrl, "https://")
			repos[i].Host = strings.Split(hostWithPath, "/")[0]
		}
		allRepos = append(allRepos, repos...)
	}
	return allRepos
}

// リポジトリのクローン状態をチェック
func checkCloneStatus(repos []Repository) []Repository {
	for i, repo := range repos {
		if _, err := os.Stat(repo.GetClonePath()); err == nil {
			repos[i].Cloned = true
		}
	}
	return repos
}

// pecoで選択されたリポジトリを処理
func processSelectedRepository(repos []Repository, selected string) {
	for _, repo := range repos {
		repoLine := formatRepoLine(repo)
		trimmedRepoLine := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(repoLine), "✓"))
		trimmedSelected := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(selected), "✓"))

		if trimmedRepoLine == trimmedSelected {
			if !repo.Cloned {
				cmd := exec.Command("ghq", "get", repo.GetGitURL())
				if output, err := cmd.CombinedOutput(); err != nil {
					handleError(err, fmt.Sprintf("リポジトリのクローンに失敗\nOutput: %s", string(output)))
				}
			}
			fmt.Println(repo.GetClonePath())
			return
		}
	}
}

func main() {
	client, err := api.DefaultRESTClient()
	handleError(err, "GitHub APIクライアントの初期化に失敗")

	orgs := fetchOrganizations(client)
	allRepos := fetchRepositories(client, orgs)
	allRepos = checkCloneStatus(allRepos)

	// pecoに渡す文字列を準備
	var lines []string
	for _, repo := range allRepos {
		lines = append(lines, formatRepoLine(repo))
	}

	// pecoコマンドを実行
	cmd := exec.Command("peco")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	handleError(err, "pecoの実行に失敗")

	selected := strings.TrimSpace(string(out))
	if selected == "" {
		fmt.Println("選択されていません")
		return
	}

	processSelectedRepository(allRepos, selected)
}
