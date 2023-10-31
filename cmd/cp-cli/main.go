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
		Name:                 "cp-cli",
		Usage:                "A computing provider cli is a client tool for managing LAG tasks.",
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
			taskCmd,
		},
	}
	app.Setup()

	if err := app.Run(os.Args); err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
	}
}
