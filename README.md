# Github config

This repository contains config files common to implementation and language-family
CNBs.

## Rules

Run [scripts/sanity.sh](scripts/sanity.sh) to see if the changes you made to this repo are valid.

Run [scripts/repo_rules.sh](scripts/repo_rules.sh) to see if your paketo cnb
github repo has recommended settings.

## How do I consume this common config

If you just wrote a new CNB, run [bootstrap.sh](scripts/bootstrap.sh) as follows:
```sh
# type is either "implementation", "language-family", or "builder"
./scripts/bootstrap.sh --target <path/to/your/cnb> --repo-type <type>
```

This will copy the relevant config files to your CNB. Git commit and Push your CNB.

Now, to wire up your CNB repo to receive relevant updates as a pull requests:
* Append your repo name to the relevant file [here](.github/data)
* Configure deploy-keys, secrets as required in workflow

Submit your change to this repo as a PR. You should be all set when the PR is merged.

## Dependency updates
Repositories managed by `github-config` get dependency updates from [Renovate](https://github.com/renovatebot/renovate). Onboarding new repositories need steps below:

For the github-config repository:
- Add the repository name to [`.github/renovate-config.js`](.github/renovate-config.js)

For the repository you are adding renovate:
- Remove current dependency update configuration `.github/dependabot.yml`
- Add `dependabot.yml` and `renovate.json` to `.github/.syncignore`
- Add `renovate.json` to the `.github` directory with following content:
```json
{
    "$schema": "https://docs.renovatebot.com/renovate-schema.json",
    "extends": [
        "github>paketo-buildpacks/github-config//renovate/<respective configuration file in github-config>"
    ]
}
```
You can add additional configuration (like repository-specific labels depending on minor, major or patch updates) to the repository owned `renovate.json`. The documentation for all configuration options can be found here: https://docs.renovatebot.com/configuration-options/
