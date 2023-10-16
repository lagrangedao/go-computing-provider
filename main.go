package main

import (
	"github.com/filswan/go-swan-lib/logs"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	cors "github.com/itsjamie/gin-cors"
	"github.com/lagrangedao/go-computing-provider/common"
	"github.com/lagrangedao/go-computing-provider/conf"
	"github.com/lagrangedao/go-computing-provider/initializer"
	"github.com/lagrangedao/go-computing-provider/routers"
	"github.com/urfave/cli/v2"
	"os"
	"strconv"
	"time"
)

const (
	FlagCpRepo = "cp-repo"
)

func main() {
	app := &cli.App{
		Name:                 "computing-provider",
		Usage:                "A computing provider is an individual or organization that participates in the decentralized computing network by offering computational resources such as processing power (CPU and GPU), memory, storage, and bandwidth.",
		EnableBashCompletion: true,
		Version:              version(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    FlagCpRepo,
				EnvVars: []string{"CP_PATH"},
				Usage:   "cp repo path",
				Value:   "~/.swan/computing",
				Hidden:  true,
			},
		},
		Commands: []*cli.Command{
			runCmd,
		},
	}
	app.Setup()

	if err := app.Run(os.Args); err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
	}
}

var runCmd = &cli.Command{
	Name:  "run",
	Usage: "Start a cp process",
	Action: func(cctx *cli.Context) error {
		logs.GetLogger().Info("Start in computing provider mode.")
		initializer.ProjectInit()

		r := gin.Default()
		r.Use(cors.Middleware(cors.Config{
			Origins:         "*",
			Methods:         "GET, PUT, POST, DELETE",
			RequestHeaders:  "Origin, Authorization, Content-Type",
			ExposedHeaders:  "",
			MaxAge:          50 * time.Second,
			ValidateHeaders: false,
		}))
		pprof.Register(r)

		v1 := r.Group("/api/v1")
		routers.CPManager(v1.Group("/computing"))

		shutdownChan := make(chan struct{})
		httpStopper, err := common.ServeHttp(r, "cp-api", ":"+strconv.Itoa(conf.GetConfig().API.Port))
		if err != nil {
			logs.GetLogger().Fatal("failed to start cp-api endpoint: %s", err)
		}

		finishCh := common.MonitorShutdown(shutdownChan,
			common.ShutdownHandler{Component: "cp-api", StopFunc: httpStopper},
		)
		<-finishCh

		return nil
	},
}

const BuildVersion = "0.2.0"

func version() string {
	return BuildVersion
}
