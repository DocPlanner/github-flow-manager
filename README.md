# github-flow-manager
## Help
```
Main goal for that app is to push commits between branches
but just those which pass evaluation checks.
Example use case "push all commits pushed to branch develop more than 30 minutes ago to branch master"

Usage:
  github-flow-manager [OWNER] [REPOSITORY] [SOURCE_BRANCH] [DESTINATION_BRANCH] [EXPRESSION] [flags]

Flags:
  -c, --commits-number int    Number of commits to get under evaluation (>0, <=100) (default 100)
  -d, --dry-run               Don't modify repository
  -f, --force                 Use the force Luke... - Changes branch HEAD with force
  -t, --github-token string   GitHub token (can be passed also as GITHUB_TOKEN env variable
  -h, --help                  help for github-flow-manager
  -v, --verbose               Print table with commits evaluation status
```

## Example
```
GITHUB_TOKEN=xxx github-flow-manager octocat Hello-World test master "StatusSuccess == false" --verbose --dry-run
```

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