# Promotion-verdict baseline

A regression net for changes to check evaluation. It records, for a set of real repositories, the
verdict this binary reaches for every commit it examines — then lets you re-record after a change
and diff.

It exists because the failure mode in this area is silent. When a required check is not found, or is
found more times than expected, the commit simply fails evaluation and the CLI still exits 0. Unit
tests cover the evaluation logic (`github/manager_test.go`), but they cannot catch a regression in
the GraphQL query itself — a check that is no longer fetched looks exactly like a check that failed.
Only running against real repositories catches that.

## Usage

```sh
cp consumers.tsv.example consumers.tsv   # then edit: your repos, your real status_check_names
./record.sh /path/to/known-good-gfm  before/     # e.g. a binary from the latest release
# ... make your change, build a new binary ...
./record.sh ./gfm-new                 after/
diff -ru before/ after/
```

Every run is `--dry-run` and uses an expression that can never be true, so no branch head is ever
moved and no commit is ever promoted.

Read `consumers.tsv.example` before filling in `consumers.tsv` — the choice of `destination` is the
one non-obvious part, and getting it wrong silently produces empty recordings.

## Reading a diff

The column that matters is `IS_STATUS_SUCCESS`: it is computed purely from the requested
`status_check_names`, so it isolates check evaluation from everything else.

A diff is not automatically a regression. These recordings read live CI state, so a repository whose
CI moved between the two runs will differ for unrelated reasons — that is what the SHAs in each file
are for. If a repository moved, re-record it; do not reason about the diff. What matters is that
every remaining difference is one you intended and can name.

## Why this is not committed

`consumers.tsv` and the output directories are gitignored. Recorded output embeds commit messages,
author names and branch names; for private repositories none of that belongs in a public repo.
