package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/fatih/color"
	"github.com/google/go-github/v61/github"
	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/text"
	"github.com/yanskun/pflag"
	"golang.org/x/text/message"
)

func main() {
	var filterVal float64
	var helpFlag bool
	pflag.BoolVarP(&helpFlag, "help", "h", false, "Show help for command")
	pflag.Float64VarP(&filterVal, "filter", "f", 1.0, "Filter past by float value in years (e.g. 0.5)")
	pflag.Parse()

	if helpFlag {
		printHelp()
		return
	}

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Start()

	account := pflag.Arg(0)
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

	var filter time.Time
	if filterVal != 0.0 {
		filter = computeFilter(filterVal)
		repos = filterRepositories(repos, filter)
	}

	results, err := getLanguages(client, repos)
	if err != nil {
		return
	}

	languages := sumLanguages(results)

	s.Stop()

	printTable(languages)
	fmt.Printf("https:github.com/%s has %d repositories\n", account, len(repos))
	if filterVal != 0.0 {
		fmt.Printf("Last updated after %s\n", filter.Format("2006-01-02"))
	}
}

func printHelp() {
	b := color.New(color.Bold)
	b.Println("\nUSAGE")
	fmt.Println("  gh langs <command> [options]\n")
	b.Println("COMMANDS")
	fmt.Println("  account:  Get languages used by a GitHub user or organization\n")
	b.Println("OPTIONS")
	pflag.Usage()
	return
}

func computeFilter(filterVal float64) time.Time {
	var filter time.Time
	totalDays := int(filterVal * 365)
	years := -totalDays / 365
	remainingDays := totalDays % 365
	months := -remainingDays / 30
	days := -remainingDays % 30
	filter = time.Now().AddDate(years, months, days)
	return filter
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

func findAccountType(client *api.RESTClient, account string) (string, error) {
	response, err := client.Request(http.MethodGet, fmt.Sprintf("users/%s", account), nil)
	if err != nil {
		return "", err
	}

	decoder := json.NewDecoder(response.Body)
	data := github.User{}
	if err := decoder.Decode(&data); err != nil {
		return "", err
	}
	if err := response.Body.Close(); err != nil {
		return "", err
	}

	if data.GetType() == "User" {
		return "users", nil
	} else if data.GetType() == "Organization" {
		return "orgs", nil
	}
	return "", fmt.Errorf("Unknown account type: %s", data.GetType())
}

func getRepositories(client *api.RESTClient, account string) ([]github.Repository, error) {
	var repos []github.Repository
	page := 1

	t, err := findAccountType(client, account)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/%s/repos", t, account)

	for {
		url := fmt.Sprintf("%s?per_page=100&page=%d", endpoint, page)
		response, err := client.Request(http.MethodGet, url, nil)
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

func filterRepositories(repos []github.Repository, filter time.Time) []github.Repository {
	var results []github.Repository
	for _, repo := range repos {
		if repo.GetUpdatedAt().After(filter) {
			results = append(results, repo)
		}
	}
	return results
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

	t.SetColumnConfigs([]table.ColumnConfig{
		{
			Name:  "Language",
			Align: text.AlignLeft,
		},
		{
			Name:  "Lines",
			Align: text.AlignRight,
			Transformer: func(val interface{}) string {
				return p.Sprintf("%d", val)
			},
			TransformerFooter: func(val interface{}) string {
				return p.Sprintf("%d", val)
			},
		},
	})

	// Append rows in sorted order
	sumLines := 0
	for _, pair := range pairs {
		sumLines += pair.Value
		t.AppendRow(table.Row{pair.Key, pair.Value})
	}

	t.AppendFooter(table.Row{"Total", sumLines})
	t.Render()
}

// For more examples of using go-gh, see:
// https://github.com/cli/go-gh/blob/trunk/example_gh_test.go
