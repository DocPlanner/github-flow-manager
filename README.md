<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [github-flow-manager](#github-flow-manager)
  - [Help](#help)
  - [Example](#example)
  - [Pre commit](#pre-commit)
- [Expressions](#expressions)
    - [Available variables](#available-variables)
    - [Examples](#examples)
  - [How to build](#how-to-build)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# github-flow-manager
## Help
```
Main goal for that app is to push commits between branches
but just those which pass evaluation checks.
Example use case "push all commits pushed to branch develop more than 30 minutes ago to branch master"

Usage:
  github-flow-manager [OWNER] [REPOSITORY] [SOURCE_BRANCH] [DESTINATION_BRANCH] [EXPRESSION] [SPECIFIC_COMMIT_CHECK_NAME - Optional] [flags]

Flags:
  -c, --commits-number int    Number of commits to get under evaluation (>0, <=100) (default 100)
  -d, --dry-run               Don't modify repository
  -f, --force                 Use the force Luke... - Changes branch HEAD with force
  -t, --github-token string   GitHub token (can be passed also as GITHUB_TOKEN env variable
  -h, --help                  help for github-flow-manager
  -v, --verbose               Print table with commits evaluation status
  -s, --separator             Set string separator of status checks (default ,)
```

## Example
- Evaluating commit status success based on the cumulative commit checks result
```
GITHUB_TOKEN=xxx github-flow-manager octocat Hello-World test master "StatusSuccess == false" --verbose --dry-run
```
- Passing specific commit check name for the evaluation of the status success of the commit
```
GITHUB_TOKEN=xxx github-flow-manager octocat Hello-World test master "StatusSuccess == false" "pipeline-name-to-be-checked" --verbose --dry-run
GITHUB_TOKEN=xxx github-flow-manager octocat Hello-World test master "StatusSuccess == false" "pipeline-1-name-to-be-checked,pipeline-2-name-to-be-checked" --verbose --dry-run
```

## Pre commit

This repo leverage pre commit to lint, secure, document the IaaC codebase. The pre-commit configuration require the following dependencies:
- [pre-commit](https://pre-commit.com/#install)
- [golangci-lint](https://golangci-lint.run/usage/install/#local-installation)

**One first repo download, to install the pre-commit hooks run**: `pre-commit install`, to run the hooks at will run: `pre-commit run -a`

# Expressions
### Available variables
 - `SHA`
 - `Message`
 - `PushedDate` - when commit was pushed
 - `StatusSuccess` - f.ex. CI status

### Examples
##### Commit message contains "string"
`Message contains "HOTFIX"`
##### Commit message not contains "string"
`Message NOT contains "FEATURE"`
##### Commit message equals "string"
`Message == "very important commit"`
##### Commit status is SUCCESS
`StatusSuccess == true`
##### Commit was pushed more than 30 minutes ago
`PushedDate < "now-30m"`
##### Commit was pushed more than 30 minutes ago and status is SUCCESS
`PushedDate < "now-30m" AND StatusSuccess == true`

## How to build

You will need:
* `goreleaser` binary
* Github access using token
* Docker image push access

Check tags
```
git tag
```

Tag your changes
```
git tag -a v1.0.X -m "fix"
```

Release
Note: you should generate your Personal Access Token with `write packages` scope enabled
```
GITHUB_TOKEN=<PERSONAL_ACCESS_TOKEN> goreleaser release
```
