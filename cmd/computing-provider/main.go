package main

import (
	"github.com/lagrangedao/go-computing-provider/build"
	"github.com/urfave/cli/v2"
	"os"
)

const (
	FlagCpRepo = "cp-repo"
)

func main() {
	app := &cli.App{
		Name:                 "computing-provider",
		Usage:                "A computing provider is an individual or organization that participates in the decentralized computing network by offering computational resources such as processing power (CPU and GPU), memory, storage, and bandwidth.",
		EnableBashCompletion: true,
		Version:              build.UserVersion(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    FlagCpRepo,
				EnvVars: []string{"CP_PATH"},
				Usage:   "cp repo path",
				Value:   "~/.swan/computing",
			},
		},
		Commands: []*cli.Command{
			runCmd,
			taskCmd,
		},
	}
	app.Setup()

	if err := app.Run(os.Args); err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
	}
}
