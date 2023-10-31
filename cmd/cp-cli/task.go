package main

import (
	"github.com/filecoin-project/lotus/lib/tablewriter"
	"github.com/lagrangedao/go-computing-provider/util"
	"github.com/urfave/cli/v2"
	"os"
)

var taskCmd = &cli.Command{
	Name:  "task",
	Usage: "Manage tasks with cp-cli",
	Subcommands: []*cli.Command{
		taskList,
		taskDetail,
		taskDelete,
	},
}

var taskList = &cli.Command{
	Name:  "list",
	Usage: "List task",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "id",
			Usage:   "Output ID addresses",
			Aliases: []string{"i"},
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := util.ReqContext()

		// List of Maps whose keys are defined above. One row = one list element = one task
		var wallets []map[string]interface{}

		// Init the tablewriter's columns
		tw := tablewriter.New(
			tablewriter.Col(addressKey),
			tablewriter.Col(idKey),
			tablewriter.Col(balanceKey),
			tablewriter.Col(marketAvailKey),
			tablewriter.Col(marketLockedKey),
			tablewriter.Col(nonceKey),
			tablewriter.Col(defaultKey),
			tablewriter.NewLineCol(errorKey))
		// populate it with content
		for _, wallet := range wallets {
			tw.Write(wallet)
		}
		// return the corresponding string
		return tw.Flush(os.Stdout)

	},
}

var taskDetail = &cli.Command{
	Name:  "get",
	Usage: "Get task detail info",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "id",
			Usage:   "Output ID addresses",
			Aliases: []string{"i"},
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := util.ReqContext()

		return nil
	},
}

var taskDelete = &cli.Command{
	Name:  "delete",
	Usage: "Delete an task from the k8s",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "id",
			Usage:   "Output ID addresses",
			Aliases: []string{"i"},
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := util.ReqContext()

		return nil
	},
}
