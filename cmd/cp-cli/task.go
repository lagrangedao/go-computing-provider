package main

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/lagrangedao/go-computing-provider/conf"
	"github.com/lagrangedao/go-computing-provider/constants"
	"github.com/lagrangedao/go-computing-provider/internal/computing"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
	"os"
	"time"
)

var taskCmd = &cli.Command{
	Name:  "task",
	Usage: "Manage tasks with cp-cli",
	Subcommands: []*cli.Command{
		taskList,
		//taskDetail,
		//taskDelete,
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
		cpPath, exit := os.LookupEnv("CP_PATH")
		if !exit {
			return fmt.Errorf("missing CP_PATH env, please set export CP_PATH=xxx")
		}
		if err := conf.InitConfig(cpPath); err != nil {
			return fmt.Errorf("load config file failed, error: %+v", err)
		}

		conn := computing.GetRedisClient()
		prefix := constants.REDIS_FULL_PREFIX + "*"
		keys, err := redis.Strings(conn.Do("KEYS", prefix))
		if err != nil {
			return fmt.Errorf("failed get redis %s prefix, error: %+v", prefix, err)
		}

		var taskData [][]string
		var rowColorList []RowColor
		var number int
		for _, key := range keys {
			jobDetail, err := computing.RetrieveJobMetadata(key)
			if err != nil {
				return fmt.Errorf("failed get job detail: %s, error: %+v", key, err)
			}
			et := time.Unix(jobDetail.ExpireTime, 0).Format("2006-01-02 15:04:05")

			k8sService := computing.NewK8sService()
			status, err := k8sService.GetDeploymentStatus(jobDetail.WalletAddress, jobDetail.SpaceUuid)
			if err != nil {
				return fmt.Errorf("failed get job status: %s, error: %+v", jobDetail.JobUuid, err)
			}

			taskData = append(taskData, []string{jobDetail.JobUuid, jobDetail.TaskType, jobDetail.WalletAddress, jobDetail.SpaceName, jobDetail.DeployName, status, "SPACE STATUS", "RTD", et, ""})

			var rowColor []tablewriter.Colors
			switch status {
			case "Pending":
				rowColor = []tablewriter.Colors{{tablewriter.Bold, tablewriter.FgYellowColor}, {tablewriter.Bold, tablewriter.FgWhiteColor}}
			case "Running":
				rowColor = []tablewriter.Colors{{tablewriter.Bold, tablewriter.FgGreenColor}, {tablewriter.Bold, tablewriter.FgWhiteColor}}
			case "Failed":
				fallthrough
			case "Unknown":
				rowColor = []tablewriter.Colors{{tablewriter.Bold, tablewriter.FgRedColor}, {tablewriter.Bold, tablewriter.FgWhiteColor}}
			}

			rowColorList = append(rowColorList, RowColor{
				row:    number,
				column: []int{6},
				color:  rowColor,
			})

			number++
		}

		header := []string{"TASK UUID", "TASK TYPE", "WALLET ADDRESS", "SPACE NAME", "DEPLOY NAME", "STATUS", "SPACE STATUS", "RTD", "ET", "REWARDS"}

		NewVisualTable(header, taskData, []RowColor{
			{
				row:    number,
				column: []int{1, 3},
				color:  []tablewriter.Colors{{tablewriter.Normal, tablewriter.FgRedColor}, {tablewriter.Bold, tablewriter.FgWhiteColor}},
			},
		}).Generate()

		return nil

	},
}

//var taskDetail = &cli.Command{
//	Name:  "get",
//	Usage: "Get task detail info",
//	Flags: []cli.Flag{
//		&cli.BoolFlag{
//			Name:    "id",
//			Usage:   "Output ID addresses",
//			Aliases: []string{"i"},
//		},
//	},
//	Action: func(cctx *cli.Context) error {
//		ctx := util.ReqContext()
//
//		return nil
//	},
//}
//
//var taskDelete = &cli.Command{
//	Name:  "delete",
//	Usage: "Delete an task from the k8s",
//	Flags: []cli.Flag{
//		&cli.BoolFlag{
//			Name:    "id",
//			Usage:   "Output ID addresses",
//			Aliases: []string{"i"},
//		},
//	},
//	Action: func(cctx *cli.Context) error {
//		ctx := util.ReqContext()
//
//		return nil
//	},
//}
