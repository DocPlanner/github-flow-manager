[![Tests][tests-badge]][tests-link]
[![GitHub Release][release-badge]][release-link]
[![Go Report Card][report-badge]][report-link]
[![License][license-badge]][license-link]

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**

- [github-flow-manager](#github-flow-manager)
  - [Help](#help)
  - [Example](#example)
    - [How status check names are resolved](#how-status-check-names-are-resolved)
  - [Pre commit](#pre-commit)
  - [Expressions](#expressions)
    - [Available variables](#available-variables)
  - [Examples](#examples)
    - [Commit message contains "string"](#commit-message-contains-string)
    - [Commit message not contains "string"](#commit-message-not-contains-string)
    - [Commit message equals "string"](#commit-message-equals-string)
    - [Commit status is SUCCESS](#commit-status-is-success)
    - [Commit was authored more than 30 minutes ago](#commit-was-authored-more-than-30-minutes-ago)
    - [Commit was authored more than 30 minutes ago and status is SUCCESS](#commit-was-authored-more-than-30-minutes-ago-and-status-is-success)
  - [How to build](#how-to-build)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# github-flow-manager

## Help

```sh
Main goal for that app is to push commits between branches
but just those which pass evaluation checks.
Example use case "push all commits pushed to branch develop more than 30 minutes ago to branch master"

Usage:
  github-flow-manager [OWNER] [REPOSITORY] [SOURCE_BRANCH] [DESTINATION_BRANCH] [EXPRESSION] [SPECIFIC_COMMIT_CHECK_NAME - Optional] [flags]

Flags:
  -c, --commits-number int      Number of commits to get under evaluation (>0, <=100) (default 100)
      --check-suites-number int Check suites to fetch per commit in the first page (>0, <=100) (default 50)
      --contexts-number int     Status checks to fetch per commit in the first page (>0, <=100) (default 50)
  -d, --dry-run                 Don't modify repository
  -f, --force                   Use the force Luke... - Changes branch HEAD with force
  -t, --github-token string     GitHub token (can be passed also as GITHUB_TOKEN env variable
  -h, --help                    help for github-flow-manager
  -v, --verbose                 Print table with commits evaluation status
  -s, --separator               Set string separator of status checks (default ,)
```

`--contexts-number` and `--check-suites-number` size the *first* page of check data fetched per
commit. GitHub caps both connections at 100 per page; anything beyond the first page is fetched on
demand, only for commits that are examined and have not already passed. So these flags trade the
number of API calls against the size of each one — lowering them does not cause checks to be missed.

On a repository with a lot of CI, one request can take longer than GitHub is willing to spend on it
and you get a `502` or `504` back instead of an answer. The server-side work is roughly
`--commits-number × (--contexts-number + --check-suites-number)`, so lowering any of the three helps;
lower `--commits-number` last, since that one does change which commits are considered.

## Example

- Evaluating commit status success based on the cumulative commit checks result

```sh
GITHUB_TOKEN=xxx github-flow-manager octocat Hello-World test master "StatusSuccess == false" --verbose --dry-run
```

- Passing specific commit check name for the evaluation of the status success of the commit

```sh
GITHUB_TOKEN=xxx github-flow-manager octocat Hello-World test master "StatusSuccess == false" "pipeline-name-to-be-checked" --verbose --dry-run
GITHUB_TOKEN=xxx github-flow-manager octocat Hello-World test master "StatusSuccess == false" "pipeline-1-name-to-be-checked,pipeline-2-name-to-be-checked" --verbose --dry-run
```

### How status check names are resolved

Each name you pass is looked for in three places, because GitHub exposes checks in three different
shapes and which one a given name lands in is not always obvious:

- a **classic commit status** (what the Statuses API sets)
- a **check run** (what a GitHub Actions job or a third-party app publishes)
- a **workflow name**, in which case the conclusion of that workflow's check suite decides

`StatusSuccess` is true only when *every* name you passed resolved to a success. A name is satisfied
if any of the above succeeded for it, so it is safe for the same name to appear in more than one
shape, and for a check to have been re-run several times — neither counts against the commit.

Only the literal conclusion `SUCCESS` satisfies a name. `NEUTRAL` and `SKIPPED` do not.

With `--verbose`, any commit that fails its checks is listed afterwards with the reason per name,
which separates the two cases that otherwise look identical:

```text
UNSATISFIED STATUS CHECKS ("not found" means no status, check run or workflow of that name exists on the commit):
  a1b2c3d4  failed: Backend tests
  e5f6a7b8  not found: Backend tests
```

`not found` almost always means the configured name no longer matches anything the CI publishes —
a different fix from a genuinely failing build.

## Pre commit

This repo leverage pre commit to lint, secure, document the IaaC codebase. The pre-commit configuration require the following dependencies:

- [pre-commit](https://pre-commit.com/#install)
- [golangci-lint](https://golangci-lint.run/usage/install/#local-installation)

**One first repo download, to install the pre-commit hooks run**: `pre-commit install`, to run the hooks at will run: `pre-commit run -a`

## Expressions

### Available variables

- `SHA`
- `Message`
- `AuthoredDate` - when commit was authored
- `StatusSuccess` - f.ex. CI status

## Examples

### Commit message contains "string"

`Message contains "HOTFIX"`

### Commit message not contains "string"

`Message NOT contains "FEATURE"`

### Commit message equals "string"

`Message == "very important commit"`

### Commit status is SUCCESS

`StatusSuccess == true`

### Commit was authored more than 30 minutes ago

`AuthoredDate < "now-30m"`

### Commit was authored more than 30 minutes ago and status is SUCCESS

`AuthoredDate < "now-30m" AND StatusSuccess == true`

## How to build

You will need:

- Permissions to create tags in `master` branch

Check tags

```sh
git tag
```

Tag your changes to create a new release with the tag specified:

```sh
git tag -a v1.0.X -m "fix"
```

<!-- JUST BADGES & LINKS -->
[tests-badge]: https://img.shields.io/github/workflow/status/DocPlanner/github-flow-manager/Tests
[tests-link]: https://github.com/DocPlanner/github-flow-manager/actions?query=workflow%3ATests

[release-badge]: https://img.shields.io/github/release/DocPlanner/github-flow-manager.svg?logo=github&labelColor=262b30
[release-link]: https://github.com/DocPlanner/github-flow-manager/releases

[report-badge]: https://goreportcard.com/badge/github.com/DocPlanner/github-flow-manager
[report-link]: https://goreportcard.com/report/github.com/DocPlanner/github-flow-manager

[license-badge]: https://img.shields.io/github/license/DocPlanner/github-flow-manager
[license-link]: https://github.com/DocPlanner/github-flow-manager/blob/master/LICENSE
