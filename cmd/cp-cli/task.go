package main

import (
	"context"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/lagrangedao/go-computing-provider/conf"
	"github.com/lagrangedao/go-computing-provider/constants"
	"github.com/lagrangedao/go-computing-provider/internal/computing"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	"os"
	"strings"
	"time"
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
			Name:    "verpose",
			Usage:   "--verpose",
			Aliases: []string{"v"},
		},
	},
	Action: func(cctx *cli.Context) error {

		fullFlag := cctx.Bool("verpose")

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

			if fullFlag {
				var spaceUuid string
				if len(jobDetail.DeployName) > 0 {
					spaceUuid = jobDetail.DeployName[7:]
				}
				taskData = append(taskData,
					[]string{jobDetail.JobUuid, jobDetail.TaskType, jobDetail.WalletAddress, spaceUuid, jobDetail.SpaceName, jobDetail.DeployName, status, "SPACE STATUS", "RTD", et, ""})
			} else {

				var walletAddress string
				if len(jobDetail.WalletAddress) > 0 {
					walletAddress = jobDetail.WalletAddress[24:]
				}

				var jobUuid string
				if len(jobDetail.JobUuid) > 0 {
					jobUuid = jobDetail.JobUuid[24:]
				}

				var spaceUuid string
				var deployName string
				if len(jobDetail.DeployName) > 0 {
					spaceUuid = jobDetail.DeployName[:15]
					deployName = jobDetail.DeployName[32:]
				}

				taskData = append(taskData,
					[]string{jobUuid, jobDetail.TaskType, walletAddress, spaceUuid, jobDetail.SpaceName, deployName, status, "SPACE STATUS", "RTD", et, ""})
			}

			var rowColor []tablewriter.Colors
			if status == "Pending" {
				rowColor = []tablewriter.Colors{{tablewriter.Bold, tablewriter.FgYellowColor}, {tablewriter.Bold, tablewriter.FgWhiteColor}}
			} else if status == "Running" {
				rowColor = []tablewriter.Colors{{tablewriter.Bold, tablewriter.FgGreenColor}, {tablewriter.Bold, tablewriter.FgWhiteColor}}
			} else {
				rowColor = []tablewriter.Colors{{tablewriter.Bold, tablewriter.FgRedColor}, {tablewriter.Bold, tablewriter.FgWhiteColor}}
			}
			rowColorList = append(rowColorList, RowColor{
				row:    number,
				column: []int{6},
				color:  rowColor,
			})

			number++
		}

		header := []string{"TASK UUID", "TASK TYPE", "WALLET ADDRESS", "SPACE UUID", "SPACE NAME", "DEPLOY NAME", "STATUS", "SPACE STATUS", "RTD", "ET", "REWARDS"}

		NewVisualTable(header, taskData, rowColorList).Generate()

		return nil

	},
}

var taskDetail = &cli.Command{
	Name:      "get",
	Usage:     "Get task detail info",
	ArgsUsage: "[space uuid]",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 1 {
			return fmt.Errorf("incorrect number of arguments, got %d", cctx.NArg())
		}

		cpPath, exit := os.LookupEnv("CP_PATH")
		if !exit {
			return fmt.Errorf("missing CP_PATH env, please set export CP_PATH=xxx")
		}
		if err := conf.InitConfig(cpPath); err != nil {
			return fmt.Errorf("load config file failed, error: %+v", err)
		}
		computing.GetRedisClient()

		spaceUuid := constants.REDIS_FULL_PREFIX + cctx.Args().First()
		jobDetail, err := computing.RetrieveJobMetadata(spaceUuid)
		if err != nil {
			return fmt.Errorf("failed get job detail: %s, error: %+v", spaceUuid, err)
		}
		et := time.Unix(jobDetail.ExpireTime, 0).Format("2006-01-02 15:04:05")

		k8sService := computing.NewK8sService()
		status, err := k8sService.GetDeploymentStatus(jobDetail.WalletAddress, jobDetail.SpaceUuid)
		if err != nil {
			return fmt.Errorf("failed get job status: %s, error: %+v", jobDetail.JobUuid, err)
		}

		var taskData [][]string
		taskData = append(taskData, []string{"TASK TYPE:", jobDetail.TaskType})
		taskData = append(taskData, []string{"WALLET ADDRESS:", jobDetail.WalletAddress})
		taskData = append(taskData, []string{"SPACE NAME:", jobDetail.SpaceName})
		taskData = append(taskData, []string{"REWARD:", ""})
		taskData = append(taskData, []string{"HARDWARE:", ""})
		taskData = append(taskData, []string{"STATUS:", status})
		taskData = append(taskData, []string{"DEPLOY NAME:", jobDetail.DeployName})
		taskData = append(taskData, []string{"RTD:", ""})
		taskData = append(taskData, []string{"ET:", et})

		var rowColor []tablewriter.Colors
		if status == "Pending" {
			rowColor = []tablewriter.Colors{{tablewriter.Bold, tablewriter.FgYellowColor}, {tablewriter.Bold, tablewriter.FgWhiteColor}}
		} else if status == "Running" {
			rowColor = []tablewriter.Colors{{tablewriter.Bold, tablewriter.FgGreenColor}, {tablewriter.Bold, tablewriter.FgWhiteColor}}
		} else {
			rowColor = []tablewriter.Colors{{tablewriter.Bold, tablewriter.FgRedColor}, {tablewriter.Bold, tablewriter.FgWhiteColor}}
		}

		header := []string{"TASK UUID:", jobDetail.JobUuid}

		var rowColorList []RowColor
		rowColorList = append(rowColorList, RowColor{
			row:    5,
			column: []int{1},
			color:  rowColor,
		})
		NewVisualTable(header, taskData, rowColorList).Generate()
		return nil
	},
}

var taskDelete = &cli.Command{
	Name:      "delete",
	Usage:     "Delete an task from the k8s",
	ArgsUsage: "[WalletAddress deploy-name]",
	Action: func(cctx *cli.Context) error {
		if cctx.NArg() != 2 {
			return fmt.Errorf("incorrect number of arguments, got %d", cctx.NArg())
		}

		cpPath, exit := os.LookupEnv("CP_PATH")
		if !exit {
			return fmt.Errorf("missing CP_PATH env, please set export CP_PATH=xxx")
		}
		if err := conf.InitConfig(cpPath); err != nil {
			return fmt.Errorf("load config file failed, error: %+v", err)
		}

		namespace := constants.K8S_NAMESPACE_NAME_PREFIX + strings.ToLower(cctx.Args().First())
		deployName := cctx.Args().Get(1)
		spaceUuid := strings.ReplaceAll(deployName, constants.K8S_DEPLOY_NAME_PREFIX, "")

		k8sService := computing.NewK8sService()
		if err := k8sService.DeleteDeployment(context.TODO(), namespace, deployName); err != nil && !errors.IsNotFound(err) {
			return err
		}
		time.Sleep(6 * time.Second)

		if err := k8sService.DeleteDeployRs(context.TODO(), namespace, spaceUuid); err != nil && !errors.IsNotFound(err) {
			return err
		}

		conn := computing.GetRedisClient()
		conn.Do("DEL", redis.Args{}.AddFlat(constants.REDIS_FULL_PREFIX+spaceUuid)...)

		return nil
	},
}
