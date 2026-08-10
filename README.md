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
  - [How specific check names are evaluated](#how-specific-check-names-are-evaluated)
  - [Fetching a commit's checks](#fetching-a-commits-checks)
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
      --accept-skipped-checks
                              Accept a required check GitHub reports as SKIPPED as satisfied
  -c, --commits-number int    Number of commits to get under evaluation (>0, <=100) (default 100)
      --check-suites-number int
                              Check suites to fetch per commit in the first page (>0, <=100) (default 50)
      --contexts-number int   Status checks to fetch per commit in the first page (>0, <=100) (default 50)
  -d, --dry-run               Don't modify repository
  -f, --force                 Use the force Luke... - Changes branch HEAD with force
  -t, --github-token string   GitHub token (can be passed also as GITHUB_TOKEN env variable
  -h, --help                  help for github-flow-manager
  -v, --verbose               Print table with commits evaluation status
  -s, --separator             Set string separator of status checks (default ,)
```

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

## How specific check names are evaluated

When `SPECIFIC_COMMIT_CHECK_NAME` is given, every name in it is evaluated
**separately**, and `StatusSuccess` is true only when **all** of them are
satisfied. A name is satisfied when:

- at least one result on the commit carries that name - otherwise the check
  never ran and the commit is not promotable;
- no result for that name is still queued or in progress, so a promotion can
  never pre-empt a verdict; and
- every concluded result for that name concluded `SUCCESS`.

Only `SUCCESS` satisfies a required check. `NEUTRAL` deliberately does **not**:
`devops-pipelines`' `hotfix_aware_skip_check` publishes `hotfix-skip-tests` as
`SUCCESS` to mean "authorised to promote without tests" and `NEUTRAL` to mean
"not a hotfix", and repositories gate their hotfix promote on that single name,
so accepting `NEUTRAL` would turn a fail-closed gate into a fail-open one.

A name may legitimately be reported **more than once** - a merge queue builds a
commit twice, once for the queue run and once for the push run, and a reusable
workflow called by several callers publishes its check once per caller. Several
results for one name are fine as long as they are all green. A name can also be
reported as a commit status context, as the name of a workflow, or as the name of
a check run; all three are searched.

`SKIPPED` does not satisfy a required check either, unless
`--accept-skipped-checks` is passed. Prefer solving a legitimate skip *upstream*,
by publishing one aggregator check that is only ever `success` or `failure` -
`dbt-app`'s `dbt-app CI merge readiness` and `noa-whisper-app`'s
`all_builds_passed` both do this - so the promotion gate never has to interpret a
skip.

Without `SPECIFIC_COMMIT_CHECK_NAME`, `StatusSuccess` comes from GitHub's own
status check rollup for the commit, unchanged.

With `--verbose`, any commit held back by its required checks is listed under the
table with one line per required name, so a stalled promotion can be diagnosed
from the job log.

## Fetching a commit's checks

GitHub returns a commit's status checks and check suites as connections capped at **100 entries per
page**, and repositories with a lot of CI exceed that. `--contexts-number` and
`--check-suites-number` size only the *first* page: anything beyond it is fetched on demand, per
commit, and only for commits that are examined and have not already been satisfied. So these flags
trade the number of requests against the size of each one — lowering them does not cause a check to
be missed, and a commit is never judged on a partial view.

They default to 50 rather than the maximum because the ceiling is not only rate limit but GitHub's
own query execution time: on a repository with enough CI, a first page of 100 × 100 comes back as
`502` or `504` instead of an answer. The server-side work is roughly
`--commits-number × (--contexts-number + --check-suites-number)`, so lowering any of the three helps.
Lower `--commits-number` last, since unlike the other two it changes which commits are considered.

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
