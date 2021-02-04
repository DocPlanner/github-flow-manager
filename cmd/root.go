package cmd

import (
	"fmt"
	"os"

	"github.com/Docplanner/github-flow-manager/flow-manager"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

var commitsNumber *int
var githubToken *string
var force *bool
var verbose *bool
var dryRun *bool

const SYMBOL_SUCCESS = "✔"
const SYMBOL_FAIL = "✖"

var rootCmd = &cobra.Command{
	Args:  cobra.MinimumNArgs(5),
	Use:   "github-flow-manager [OWNER] [REPOSITORY] [SOURCE_BRANCH] [DESTINATION_BRANCH] [EXPRESSION]",
	Short: "GitHub Flow Manager",
	Long: `Main goal for that app is to push commits between branches
but just those which pass evaluation checks. 
Example use case "push all commits pushed to branch develop more than 30 minutes ago to branch master"`,
	Run: func(cmd *cobra.Command, args []string) {
		owner := args[0]
		repo := args[1]
		sourceBranch := args[2]
		destinationBrnach := args[3]
		expression := strings.Join(args[4:], " ")

		for _, a := range args {
			if len(a) < 1 {
				cmd.Help()
				return
			}
		}

		if "" == *githubToken {
			*githubToken = os.Getenv("GITHUB_TOKEN")
			if "" == *githubToken {
				fmt.Println("Github token not set!")
				cmd.Help()
				return
			}
		}

		results, err := flow_manager.Manage(*githubToken, owner, repo, sourceBranch, destinationBrnach, expression, *commitsNumber, *force, *dryRun)
		if nil != err {
			fmt.Println(err.Error())
			os.Exit(1)
			return
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

			statusSign := SYMBOL_FAIL
			if c.StatusSuccess {
				statusSign = SYMBOL_SUCCESS
			}

			resultSign := SYMBOL_FAIL
			if res.Result {
				resultSign = SYMBOL_SUCCESS
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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	commitsNumber = rootCmd.Flags().IntP("commits-number", "c", 100, "Number of commits to get under evaluation (>0, <=100)")
	githubToken = rootCmd.Flags().StringP("github-token", "t", "", "GitHub token (can be passed also as GITHUB_TOKEN env variable")
	force = rootCmd.Flags().BoolP("force", "f", false, "Use the force Luke... - Changes branch HEAD with force")
	verbose = rootCmd.Flags().BoolP("verbose", "v", false, "Print table with commits evaluation status")
	dryRun = rootCmd.Flags().BoolP("dry-run", "d", false, "Don't modify repository")
}
