package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/docopt/docopt-go"
	pb "github.com/nogproject/nog/backend/pkg/nogechopb"
	"google.golang.org/grpc"
)

// `xVersion` and `xBuild` are injected by the `Makefile`.
var (
	xVersion string
	xBuild   string
	version  = fmt.Sprintf("nogecho-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogecho [options] <msg>

Options:
  --server=<addr>  [default: 127.0.0.1:7540]
`)

func main() {
	args := argparse()
	addr := args["--server"].(string)
	msg := args["<msg>"].(string)

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	c := pb.NewEchoClient(conn)
	req := pb.EchoRequest{Message: msg}
	res, err := c.Echo(context.Background(), &req)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Message)
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
