package cmd

import (
	"fmt"
	"os"

	"strings"
	"time"

	flow_manager "github.com/Docplanner/github-flow-manager/manager"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	commitsNumber *int
	githubToken   *string
	force         *bool
	verbose       *bool
	dryRun        *bool
	separator     *string
)

const (
	// symbolSuccess represents the symbol for successful
	symbolSuccess = "✔"
	// symbolSuccess represents the symbol for failure
	symbolFail = "✖"
)

// checkMinimumArgs check that is set the minimum args to call the cobra command
func checkMinimumArgs(args []string) error {
	for _, a := range args {
		if len(a) < 1 {
			return fmt.Errorf("not provided minimum args")
		}
	}
	return nil
}

// rootCmd represents the root command
var rootCmd = &cobra.Command{
	Args:  cobra.MinimumNArgs(5),
	Use:   "github-flow-manager [OWNER] [REPOSITORY] [SOURCE_BRANCH] [DESTINATION_BRANCH] [EXPRESSION] [SPECIFIC_COMMIT_CHECK_NAME]",
	Short: "GitHub Flow Manager",
	Long: `Main goal for that app is to push commits between branches
but just those which pass evaluation checks.
Example use case "push all commits pushed to branch develop more than 30 minutes ago to branch master"
If a SPECIFIC_COMMIT_CHECK_NAME is specified, the StatusSuccess will be calculated based ONLY on the result of that specific commit check`,
	Run: func(cmd *cobra.Command, args []string) {
		owner := args[0]
		repo := args[1]
		sourceBranch := args[2]
		destinationBranch := args[3]
		expression := strings.Join(args[4:], " ")
		specificChecksNames := ""
		if len(args) > 5 {
			specificChecksNames = args[5]
		}

		err := checkMinimumArgs(args)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if *githubToken == "" {
			*githubToken = os.Getenv("GITHUB_TOKEN")
		}

		results, err := flow_manager.Manage(*githubToken, owner, repo, sourceBranch, destinationBranch, expression, specificChecksNames, *separator, *commitsNumber, *force, *dryRun)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if !*verbose {
			return
		}

		if len(results) < 1 {
			fmt.Println("No commits on source branch")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetRowLine(true)
		table.SetHeader([]string{"SHA", "MESSAGE", "PUSHED_AT", "IS_STATUS_SUCCESS", "EVALUATION"})

		for _, res := range results {
			c := res.Commit

			statusSign := symbolFail
			if c.StatusSuccess {
				statusSign = symbolSuccess
			}

			resultSign := symbolFail
			if res.Result {
				resultSign = symbolSuccess
			}

			table.Append([]string{c.SHA, c.Message, c.PushedDate.Format(time.RFC3339), statusSign, resultSign})
		}

		table.Render()

		endingMessage := "THERE IS NO COMMITS PASSING EVALUATION"
		if results[len(results)-1].Result {
			endingMessage = "NO MORE COMMITS WERE EXAMINED BECAUSE LAST ONE EVALUATED SUCCESSFULLY"
		}

		fmt.Println("\t!!!!! " + endingMessage + " !!!!!")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	commitsNumber = rootCmd.Flags().IntP("commits-number", "c", 100, "Number of commits to get under evaluation (>0, <=100)")
	githubToken = rootCmd.Flags().StringP("github-token", "t", "", "GitHub token (can be passed also as GITHUB_TOKEN env variable")
	force = rootCmd.Flags().BoolP("force", "f", false, "Use the force Luke... - Changes branch HEAD with force")
	verbose = rootCmd.Flags().BoolP("verbose", "v", false, "Print table with commits evaluation status")
	dryRun = rootCmd.Flags().BoolP("dry-run", "d", false, "Don't modify repository")
	separator = rootCmd.Flags().StringP("separator", "s", ",", "Set string separator of status checks")
}
