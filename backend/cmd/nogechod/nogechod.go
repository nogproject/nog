package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/internal/nogechod"
	pb "github.com/nogproject/nog/backend/pkg/nogechopb"
	"google.golang.org/grpc"
)

// `xVersion` and `xBuild` are injected by the `Makefile`.
var (
	xVersion string
	xBuild   string
	version  = fmt.Sprintf("nogechod-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogechod [options]

Options:
  --listen=<addr>  [default: 0.0.0.0:7540]
`)

func logf(format string, a ...interface{}) string {
	return fmt.Sprintf("[nogechod] "+format, a...)
}

func main() {
	args := argparse()
	addr := args["--listen"].(string)

	log.Print(logf("Started."))

	gsrv := grpc.NewServer()
	pb.RegisterEchoServer(gsrv, &nogechod.Server{})

	addrType := "tcp"
	if strings.HasPrefix(addr, "/") {
		addrType = "unix"
		os.Remove(addr)
	}
	lis, err := net.Listen(addrType, addr)
	if err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	isShutdown := false
	gracePeriod := 20 * time.Second
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGTERM)

	go func() {
		err := gsrv.Serve(lis)
		if isShutdown {
			done <- true
			return
		}
		log.Fatal(err)
	}()

	log.Print(logf(
		"Listening on `%s:%s`.", addrType, addr,
	))

	<-sigs
	isShutdown = true
	gsrv.GracefulStop()
	timeout := time.NewTimer(gracePeriod)
	log.Print(
		logf("SIGTERM, started %s graceful shutdown.", gracePeriod),
	)

	select {
	case <-timeout.C:
		gsrv.Stop()
		log.Print(logf("Timeout, forced shutdown."))
	case <-done:
	}

}

func argparse() map[string]interface{} {
	const autoHelp = true
	const noOptionFirst = false
	args, err := docopt.Parse(
		usage, nil, autoHelp, version, noOptionFirst,
	)
	if err != nil {
		panic(fmt.Sprintf("docopt failed: %s", err))
	}
	return args
}
