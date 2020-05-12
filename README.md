# Github config

This repository contains config files common to implementation and language-family
CNBs.

## Rules

Run [scripts/sanity.sh](scripts/sanity.sh) to see if the changes you made to this repo are valid.

## How do I consume this common config

If you just wrote a new CNB, run [bootstrap.sh](scripts/bootstrap.sh) as follows:
```sh
# type is either "implementation" or "language-family"
./scripts/bootstrap.sh <path/to/your/cnb> <type>
```

This will copy the relevant config files to your CNB. Git commit and Push your CNB.

Now, to wire up your CNB repo to receive relevant updates as a pull requests:
* Append your repo name to the relevant file [here](.github/data)
* Configure deploy-keys, secrets as required in workflow
* To auto-merge these PRs, turn on [mergify](https://github.com/marketplace/mergify) for your cnb repo

Submit your change to this repo as a PR. You should be all set when the PR is merged.

