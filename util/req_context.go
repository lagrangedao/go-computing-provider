package util

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func ReqContext() context.Context {
	tCtx := context.Background()

	ctx, done := context.WithCancel(tCtx)
	sigChan := make(chan os.Signal, 2)
	go func() {
		<-sigChan
		done()
	}()
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	return ctx
}
