package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	flag "github.com/spf13/pflag"
)

var (
	defaultOwningTeamRegex = "[Mm]aintainer"
	defaultRepoRegex       = ".*"
	defaultOrgs            = []string{"paketo-buildpacks", "paketo-community"}
)

type Team struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	Org  string // Not part of the Github body but used to construct API endpoints pertaining to the team
}

type Repo struct {
	FullName string `json:"full_name"`
	RoleName string `json:"role_name"`
}

type Release struct {
	ID              int    `json:"id"`
	Draft           bool   `json:"draft"`
	TagName         string `json:"tag_name"`
	TargetCommitish string `json:"target_commitish"`
	HTMLURL         string `json:"html_url"`
	RepoFullName    string // Not part of the GitHub body but used to construct API endpoints pertaining to the Release
}

type CommitResponse struct {
	Commits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Author struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"author"`
			Message string `json:"message"`
		} `json:"commit"`
	} `json:"commits"`
}

func main() {
	var teamsFlag = flag.StringSliceP("team", "t", []string{}, "team to search for repos in")
	var orgs = flag.StringSliceP("org", "o", defaultOrgs, "org to search for teams in")
	var teamOwnerRegex = flag.String("team-owner-regex", defaultOwningTeamRegex, "regex to identify 'owning' teams")
	var repoRegexFlag = flag.String("repo-regex", defaultRepoRegex, "regex to filter repos by")
	flag.Parse()

	if teamsFlag == nil || len(*teamsFlag) == 0 {
		fatalf("must provide at least one team with --team / -t")
	}

	teamsFilter := *teamsFlag

	if orgs == nil || len(*orgs) == 0 {
		orgs = &defaultOrgs
	}

	owningTeamRegex, err := regexp.Compile(*teamOwnerRegex)
	if err != nil {
		fatal(err)
	}

	repoRegex, err := regexp.Compile(*repoRegexFlag)
	if err != nil {
		fatal(err)
	}

	_, err = exec.LookPath("gh")
	if err != nil {
		fatalf("Please install GitHub CLI (e.g. `brew install gh`)\n", err)
	}

	_, err = exec.LookPath("jq")
	if err != nil {
		fatalf("Please install jq (e.g. `brew install jq`)\n", err)
	}

	ghAuthCmd := exec.Command("gh", "auth", "status")
	err = ghAuthCmd.Run()
	if err != nil {
		fatal(err)
	}

	var allTeams []Team
	for _, org := range *orgs {
		fmt.Printf("Getting teams for org: %s\n", org)

		endpoint := fmt.Sprintf("orgs/%s/teams", org)
		orgTeams, err := ghGetAll[Team](endpoint)
		if err != nil {
			fatal(err)
		}

		for i, _ := range orgTeams {
			orgTeams[i].Org = org
		}
		allTeams = append(allTeams, orgTeams...)
	}

	var filteredTeams []Team
	for _, team := range allTeams {
		if owningTeamRegex.MatchString(team.Name) &&
			strContainsAnySubstring(strings.ToLower(team.Name), teamsFilter) {
			filteredTeams = append(filteredTeams, team)
		}
	}

	if len(filteredTeams) == 0 {
		fatalf("No teams found for orgs: %v, teams: %v, and team owner regex: '%s'", *orgs, teamsFilter, *teamOwnerRegex)
	}

	var repos []Repo
	for _, team := range filteredTeams {
		fmt.Printf("Getting repos owned by team: %s\n", team.Name)

		endpoint := fmt.Sprintf("orgs/%s/teams/%s/repos", team.Org, team.Slug)
		teamRepos, err := ghGetAll[Repo](endpoint)
		if err != nil {
			fatal(err)
		}

		var ownedRepos []Repo
		for _, repo := range teamRepos {
			if repoRegex.MatchString(repo.FullName) &&
				repo.RoleName == "admin" {
				ownedRepos = append(ownedRepos, repo)
			}
		}

		repos = append(repos, ownedRepos...)
	}

	if len(repos) == 0 {
		var teamNames []string
		for _, team := range filteredTeams {
			teamNames = append(teamNames, team.Name)
		}
		fatalf("No repos owned by teams: %v", teamNames)
	}

	var draftReleases []Release
	for _, repo := range repos {
		fmt.Printf("Getting draft releases for repo: %s\n", repo.FullName)

		endpoint := fmt.Sprintf("/repos/%s/releases", repo.FullName)
		releases, err := ghGetAll[Release](endpoint)
		if err != nil {
			fatal(err)
		}

		if len(releases) == 0 {
			fatalf("no releases found for repo: %s", repo.FullName)
		}

		var repoDraftReleases []Release
		for _, release := range releases {
			if release.Draft {
				release.RepoFullName = repo.FullName
				repoDraftReleases = append(repoDraftReleases, release)
			}
		}

		if len(repoDraftReleases) > 1 {
			fatalf("Multiple draft releases found for repo: %s", repo.FullName)
		}

		draftReleases = append(draftReleases, repoDraftReleases...)
	}

	if len(draftReleases) == 0 {
		var repoFullNames []string
		for _, repo := range repos {
			repoFullNames = append(repoFullNames, repo.FullName)
		}
		log.Printf("No draft releases found for repos - exiting: %v", repoFullNames)
		os.Exit(0)
	}

	fmt.Println()

	for _, draftRelease := range draftReleases {

		latestRelease, err := ghGetSingle[Release](fmt.Sprintf("/repos/%s/releases/latest", draftRelease.RepoFullName))
		if err != nil {
			fatal(err)
		}

		endpoint := fmt.Sprintf("/repos/%s/compare/%s...%s", draftRelease.RepoFullName, latestRelease.TagName, draftRelease.TargetCommitish)
		commitResponse, err := ghGetSingle[CommitResponse](endpoint)
		if err != nil {
			fatal(err)
		}

		// color.Blue(fmt.Sprintf("%s - draft: %s (previous: %s)\n\n", draftRelease.RepoFullName, draftRelease.TagName, latestRelease.TagName))
		color.Blue(draftRelease.RepoFullName)
		fmt.Printf("Draft release: %s (current: %s)\n\n", draftRelease.TagName, latestRelease.TagName)
		// fmt.Printf("Draft: %s\n", draftRelease.TagName)
		// fmt.Printf("Previously published: %s\n", latestRelease.TagName)
		// fmt.Printf("Commits:\n")

		// Order commits by newest first
		sort.Slice(commitResponse.Commits, func(i, j int) bool {
			return commitResponse.Commits[i].Commit.Author.Date.After(commitResponse.Commits[j].Commit.Author.Date)
		})
		for _, c := range commitResponse.Commits {
			color.Yellow("    commit %s", c.SHA)
			fmt.Printf("    Author: %s <%s>\n", c.Commit.Author.Name, c.Commit.Author.Email)
			fmt.Printf("    Date:   %v \n\n", c.Commit.Author.Date)
			fmt.Printf("        %s\n\n", firstLineOfStr(c.Commit.Message))
		}

		var response string
		fmt.Printf("Publish release for %s (y/n)? ", draftRelease.RepoFullName)
		fmt.Scanf("%s", &response)
		switch response {
		case "y", "yes", "Y", "YES":
			publishedRelease, err := publishDraftRelease(draftRelease)
			if err != nil {
				fatal(err)
			}
			color.Green("release published at: %s", publishedRelease.HTMLURL)
		default:
			color.Yellow("not publishing release for %s", draftRelease.RepoFullName)
		}
		fmt.Println()
	}
}

func publishDraftRelease(draftRelease Release) (Release, error) {
	endpoint := fmt.Sprintf("/repos/%s/releases/%d", draftRelease.RepoFullName, draftRelease.ID)
	ghPublishDraftReleaseCmd := exec.Command(
		"gh",
		"api",
		"--method", "PATCH",
		"-F", "draft=false",
		endpoint,
	)

	apiOutput, err := ghPublishDraftReleaseCmd.CombinedOutput()
	if err != nil {
		return *new(Release), err
	}

	release := *new(Release)
	err = json.Unmarshal(apiOutput, &release)
	if err != nil {
		return release, err
	}

	return release, nil

}

func strContainsAnySubstring(str string, substrings []string) bool {
	for _, substring := range substrings {
		if strings.Contains(str, substring) {
			return true
		}
	}
	return false
}

func ghGetSingle[T any](endpoint string) (T, error) {
	ghTeamsCmd := exec.Command("gh", "api", endpoint)
	apiOutput, err := ghTeamsCmd.CombinedOutput()
	if err != nil {
		return *new(T), err
	}

	t := new(T)
	err = json.Unmarshal(apiOutput, t)
	if err != nil {
		return *new(T), err
	}

	return *t, nil

}

func ghGetAll[T any](endpoint string) ([]T, error) {
	ghTeamsCmd := exec.Command("gh", "api", "--paginate", endpoint)
	apiOutput, err := ghTeamsCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	jqSlurpCmd := exec.Command("jq", "-s", ".")
	jqSlurpCmd.Stdin = bytes.NewReader(apiOutput)
	jqSlurpOutput, err := jqSlurpCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	jqAddCmd := exec.Command("jq", "add")
	jqAddCmd.Stdin = bytes.NewReader(jqSlurpOutput)
	jqAddOutput, err := jqAddCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	t := new([]T)
	err = json.Unmarshal(jqAddOutput, t)
	if err != nil {
		return nil, err
	}

	return *t, nil
}

func firstLineOfStr(str string) string {
	split := strings.Split(str, "\n")
	if len(split) < 1 {
		panic("could not split string")
	}

	return split[0]
}

func fatal(err error) {
	color.Red("%v", err)
	os.Exit(1)
}

func fatalf(format string, v ...any) {
	color.Red(format, v...)
	os.Exit(1)
}
