# Blah blah blah

# Get 50 most recent closed PRs from a single repo:
curl -H "Authorization: token $PAKETO_GITHUB_TOKEN" 'https://api.github.com/repos/paketo-buildpacks/github-config/pulls?per_page=50&state=closed' | jq -r .

# Get the URL of the commits on a PR
curl -H "Authorization: token $PAKETO_GITHUB_TOKEN" 'https://api.github.com/repos/paketo-buildpacks/github-config/pulls?per_page=2&state=closed' | jq -r '.[1] | ._links |
.commits | .href'

# Get the date and time of the last commit on a PR:
# TODO: Add logic for skipping over "Merge to main" commits
curl -H "Authorization: token $PAKETO_GITHUB_TOKEN" 'https://api.github.com/repos/paketo-buildpacks/github-config/pulls/178/commits' | jq -r '.[-1] | .commit | .author | .d
ate'

# Get the date and time of when the PR was merged:
curl -H "Authorization: token $PAKETO_GITHUB_TOKEN" 'https://api.github.com/repos/paketo-buildpacks/github-config/pulls?per_page=2&state=closed' | jq -r '.[1] | .merged_at'

# compute the difference in timestamps between PR merge and last commit:
echo $(( ($(date --date="$merge_time" +%s) - $(date --date="$last_commit" +%s))/(60)))

#TODO Ignore bot PRs
