package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/google/go-github/v61/github"
	"github.com/jedib0t/go-pretty/table"
	"golang.org/x/text/message"
)

func main() {
	flag.Parse()

	account := flag.Arg(0)
	if account == "" {
		account, _ = getGitHubUsername()
	}

	client, err := api.DefaultRESTClient()
	if err != nil {
		return
	}

	repos, err := getRepositories(client, account)
	if err != nil {
		return
	}

	results, err := getLanguages(client, repos)
	if err != nil {
		return
	}

	languages := sumLanguages(results)

	printTable(languages)
	fmt.Println(fmt.Sprintf("https:github.com/%s has %d repositories", account, len(repos)))
}

func getGitHubUsername() (string, error) {
	cmd := exec.Command("gh", "api", "user", "--jq", ".login")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func getRepositories(client *api.RESTClient, account string) ([]github.Repository, error) {
	var repos []github.Repository
	page := 1
	for {
		endpoint := fmt.Sprintf("%s?sort=updated&per_page=100&page=%d", fmt.Sprintf("users/%s/repos", account), page)
		response, err := client.Request(http.MethodGet, endpoint, nil)
		if err != nil {
			fmt.Printf("%s is not a valid GitHub username\n", account)
			return nil, err
		}

		decoder := json.NewDecoder(response.Body)
		data := []github.Repository{}
		if err := decoder.Decode(&data); err != nil {
			return nil, err
		}
		if err := response.Body.Close(); err != nil {
			return nil, err
		}

		if len(data) == 0 {
			break
		}

		repos = append(repos, data...)
		page++
	}
	return repos, nil
}

type (
	Languages     map[string]int
	LanguagesList []Languages
)

func getLanguages(client *api.RESTClient, data []github.Repository) (LanguagesList, error) {
	results := make(LanguagesList, 0, len(data))

	var wg sync.WaitGroup

	for _, repo := range data {
		wg.Add(1)
		go func(repo github.Repository) {
			defer wg.Done()

			fullName := repo.GetFullName()
			response, err := client.Request(http.MethodGet, fmt.Sprintf("repos/%s/languages", fullName), nil)
			if err != nil {
				log.Fatal(err)
				return
			}

			decoder := json.NewDecoder(response.Body)
			data := Languages{}
			if err := decoder.Decode(&data); err != nil {
				log.Fatal(err)
				return
			}

			if err := response.Body.Close(); err != nil {
				log.Fatal(err)
				return
			}

			results = append(results, data)
		}(repo)
	}
	wg.Wait()

	return results, nil
}

func sumLanguages(list LanguagesList) Languages {
	results := Languages{}

	for _, languages := range list {
		for lang, lines := range languages {
			results[lang] += lines
		}
	}

	return results
}

type Pair struct {
	Key   string
	Value int
}

func printTable(languages Languages) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	p := message.NewPrinter(message.MatchLanguage("en"))
	t.AppendHeader(table.Row{"Language", "Lines"})

	// Convert map to slice
	pairs := make([]Pair, 0, len(languages))
	for k, v := range languages {
		pairs = append(pairs, Pair{k, v})
	}

	// Sort slice in descending order by Value
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Value > pairs[j].Value
	})

	// Append rows in sorted order
	for _, pair := range pairs {
		num := p.Sprintf("%d", pair.Value)
		t.AppendRow(table.Row{pair.Key, num})
	}
	t.Render()
}

// For more examples of using go-gh, see:
// https://github.com/cli/go-gh/blob/trunk/example_gh_test.go
