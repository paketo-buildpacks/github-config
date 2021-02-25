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

