package github

import (
	"fmt"
	"strings"

	"github.com/shurcooL/githubv4"
)

// checkOutcome is the normalised outcome of a single result that GitHub
// reported for a required check name.
type checkOutcome int

const (
	// outcomePassed means the result is green, or green enough to promote.
	outcomePassed checkOutcome = iota
	// outcomePending means the result has not concluded yet, so promoting now
	// would pre-empt its verdict.
	outcomePending
	// outcomeFailed means the result concluded in a state that is not green.
	outcomeFailed
)

// checkResult is one result on a commit that carries a required check name,
// together with the raw state GitHub reported, kept for the operator-facing
// summary.
type checkResult struct {
	outcome checkOutcome
	verdict string
}

// noVerdictYet is shown for a result GitHub has not concluded yet.
const noVerdictYet = "NO_CONCLUSION_YET"

// checkSources is everything read for one commit that can carry a required check name.
//
// It is a struct rather than the query Edge because two of the three sources are paginated
// connections: GitHub caps checkSuites and statusCheckRollup.contexts at 100 entries per page, and
// repositories with a lot of CI exceed that. Later pages append to these slices and the commit is
// evaluated again, so a verdict is never reached on a partial view.
type checkSources struct {
	statuses       []Context
	rollupContexts []RollupContext
	suites         []CheckSuiteNode
}

// sourcesFromEdge pulls the three places a required check name can be reported from one commit of
// the history query.
func sourcesFromEdge(edge Edge) checkSources {
	return checkSources{
		statuses:       edge.Node.Status.Contexts,
		rollupContexts: edge.Node.StatusCheckRollup.Contexts.Nodes,
		suites:         edge.Node.CheckSuites.Nodes,
	}
}

// splitCheckNames splits the required check names supplied on the command line,
// dropping whitespace and empty entries so that a stray separator (`a,b,` or
// `a, b`) cannot silently add a name that can never be satisfied.
func splitCheckNames(specificChecksNames, sep string) []string {
	var names []string
	for _, name := range strings.Split(specificChecksNames, sep) {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			names = append(names, trimmed)
		}
	}

	return names
}

// classifyConclusion maps the conclusion of a check suite or a check run onto a
// checkOutcome.
//
// Only SUCCESS satisfies a required check, which is exactly the acceptance rule
// the previous implementation applied - this change fixes how results are
// counted, deliberately without widening what counts as green.
//
// NEUTRAL in particular must not satisfy anything. devops-pipelines'
// hotfix_aware_skip_check workflow publishes its `hotfix-skip-tests` check as
// SUCCESS to mean "this commit is authorised to promote without tests" and
// NEUTRAL to mean "not a hotfix", and repositories gate their hotfix promote on
// that single name. Accepting NEUTRAL would turn that gate from fail-closed into
// fail-open and let any commit promote untested.
//
// SKIPPED satisfies only when acceptSkipped is set, for repositories that skip a
// required job on purpose - see the --accept-skipped-checks flag.
func classifyConclusion(conclusion githubv4.String, acceptSkipped bool) checkOutcome {
	if conclusion == "" {
		// GitHub reports a null conclusion until the suite or run completes.
		return outcomePending
	}

	switch githubv4.CheckConclusionState(conclusion) {
	case githubv4.CheckConclusionStateSuccess:
		return outcomePassed
	case githubv4.CheckConclusionStateSkipped:
		if acceptSkipped {
			return outcomePassed
		}
		return outcomeFailed
	default:
		return outcomeFailed
	}
}

// classifyCheckRun classifies a single check run. Its status wins over its
// conclusion: a run that has not completed cannot have a trustworthy verdict.
func classifyCheckRun(checkRun RollupCheckRun, acceptSkipped bool) checkResult {
	if checkRun.Status != "" && githubv4.CheckStatusState(checkRun.Status) != githubv4.CheckStatusStateCompleted {
		return checkResult{outcome: outcomePending, verdict: string(checkRun.Status)}
	}

	return checkResult{outcome: classifyConclusion(checkRun.Conclusion, acceptSkipped), verdict: verdictOf(checkRun.Conclusion)}
}

// verdictOf renders a conclusion for the summary, naming the empty case.
func verdictOf(conclusion githubv4.String) string {
	if conclusion == "" {
		return noVerdictYet
	}

	return string(conclusion)
}

// classifyStatusContext classifies a single commit status context. Commit
// statuses carry no conclusion, only a state.
func classifyStatusContext(ctx Context) checkResult {
	result := checkResult{verdict: string(ctx.State)}

	switch githubv4.StatusState(ctx.State) {
	case githubv4.StatusStateSuccess:
		result.outcome = outcomePassed
	case githubv4.StatusStatePending, githubv4.StatusStateExpected:
		result.outcome = outcomePending
	default:
		// ERROR, FAILURE, or a state this build does not recognise.
		result.outcome = outcomeFailed
	}

	return result
}

// collectResults gathers every result on the commit that carries the given
// required check name.
//
// The same name can reach us through three different GitHub concepts - a commit
// status context, the name of a workflow run, and the name of a check run - so
// all three are searched. One name legitimately matching several results is
// normal: while a merge queue is active, a commit is built twice, once for the
// queue run and once for the push run, and both report the same workflow name.
func collectResults(name string, sources checkSources, acceptSkipped bool) []checkResult {
	var results []checkResult

	// A commit status set directly on the commit (the classic statuses API).
	for _, ctx := range sources.statuses {
		if githubv4.String(name) == ctx.Context {
			results = append(results, classifyStatusContext(ctx))
		}
	}

	// The status-check rollup, which carries both check runs and commit statuses as a single flat
	// list, latest-per-name. This is where a required "workflow / job" check is found.
	for _, ctx := range sources.rollupContexts {
		switch ctx.Typename {
		case "CheckRun":
			if githubv4.String(name) == ctx.CheckRun.Name {
				results = append(results, classifyCheckRun(ctx.CheckRun, acceptSkipped))
			}
		case "StatusContext":
			if githubv4.String(name) == ctx.StatusContext.Context {
				results = append(results, classifyStatusContext(Context{
					Context: ctx.StatusContext.Context,
					State:   ctx.StatusContext.State,
				}))
			}
		}
	}

	// Or the required name is the name of the workflow itself, in which case the suite's conclusion
	// is the workflow run's verdict. Suites are still read separately because a workflow whose jobs
	// were all skipped contributes no check run to the rollup at all.
	for _, checkSuite := range sources.suites {
		if (checkSuite.WorkflowRun != WorkflowRun{}) && githubv4.String(name) == checkSuite.WorkflowRun.Workflow.Name {
			conclusion := checkSuite.WorkflowRun.CheckSuite.Conclusion
			results = append(results, checkResult{
				outcome: classifyConclusion(conclusion, acceptSkipped),
				verdict: verdictOf(conclusion),
			})
		}
	}

	return results
}

// evaluateCheckName applies "nothing missing, nothing pending, nothing red" to a
// single required check name and returns whether it is satisfied plus a line for
// the summary.
func evaluateCheckName(name string, sources checkSources, acceptSkipped bool) (bool, string) {
	results := collectResults(name, sources, acceptSkipped)
	if len(results) == 0 {
		return false, fmt.Sprintf("%s: NEVER RAN - no commit status, workflow run or check run carries this name", name)
	}

	verdicts := make([]string, 0, len(results))
	var pending, failed int
	for _, result := range results {
		verdicts = append(verdicts, result.verdict)
		switch result.outcome {
		case outcomePending:
			pending++
		case outcomeFailed:
			failed++
		case outcomePassed:
		}
	}
	reported := fmt.Sprintf("%s: %d result(s) [%s]", name, len(results), strings.Join(verdicts, " "))

	switch {
	case failed > 0:
		return false, reported + fmt.Sprintf(" - %d did not pass", failed)
	case pending > 0:
		return false, reported + fmt.Sprintf(" - %d still running", pending)
	}

	return true, "OK " + reported
}

// evaluateSpecificChecks reports whether every required check name is satisfied
// on the commit, along with a per-name summary.
//
// Every name is evaluated on its own. An earlier implementation summed the
// successful results across all names and compared that total to the number of
// names, which meant a surplus on one name could silently cover a deficit on
// another - promoting commits whose required checks had never gone green - while
// a name reporting more than one successful result pushed the total past the
// exact-equality test and blocked commits where everything had in fact passed.
// Duplicate results per name are routine, which is what made both directions of
// that bug reachable: a merge queue builds a commit twice, and a reusable
// workflow called by several callers reports its check once per caller.
func evaluateSpecificChecks(sources checkSources, checkNames []string, acceptSkipped bool) (bool, []string) {
	if len(checkNames) == 0 {
		// Names were asked for but none are usable. Fail closed: an empty
		// requirement set would make every commit vacuously promotable.
		return false, []string{"no usable required check names were supplied - refusing to promote"}
	}

	statusSuccess := true
	summary := make([]string, 0, len(checkNames))
	for _, name := range checkNames {
		passed, line := evaluateCheckName(name, sources, acceptSkipped)
		if !passed {
			statusSuccess = false
		}
		summary = append(summary, line)
	}

	return statusSuccess, summary
}
