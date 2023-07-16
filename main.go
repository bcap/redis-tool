package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/alexflint/go-arg"
)

//go:embed VERSION
var version string

var description string = "" +
	"-- redis-tool\n\n" +
	"Tool to iterate redis keys in an efficient and safe manner\n" +
	"source: https://github.com/bcap/redis-tool\n"

type Args struct {
	Count        *CountArgs  `arg:"subcommand:count" help:"Counts keys based on a key name pattern"`
	Print        *PrintArgs  `arg:"subcommand:print" help:"Prints keys names based on a key name pattern"`
	Delete       *DeleteArgs `arg:"subcommand:delete" help:"Deletes keys based on a key name pattern"`
	RedisAddress string      `arg:"-a,--address,required" help:"redis server address. Eg: localhost:6379"`
	Cluster      bool        `arg:"-c,--cluster" help:"connect in cluster mode"`
}

func (*Args) Version() string {
	return version
}

func (*Args) Description() string {
	return description
}

func main() {
	args := Args{}
	parser := arg.MustParse(&args)

	if parser.Subcommand() == nil {
		parser.Fail("no command provided")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := NewUnifiedClient(ctx, args.RedisAddress)
	failOnErr(err)

	checkClusterModeInconsistency(ctx, args, client)

	switch {
	case args.Count != nil:
		err = CountCmd(ctx, parser, client, args)
	case args.Print != nil:
		err = PrintCmd(ctx, parser, client, args)
	case args.Delete != nil:
		err = DeleteCmd(ctx, parser, client, args)
	}

	failOnErr(err)
}

func checkClusterModeInconsistency(ctx context.Context, args Args, client UnifiedClient) {
	if client.IsCluster && !args.Cluster {
		log.Printf(""+
			severeWarning+" Node %s is a member of a cluster, but cluster mode (-c|--cluster) is NOT enabled. "+
			"Commands will either fail or be local to this particular node. Waiting 5 seconds before continuing",
			args.RedisAddress,
		)
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
		}
		client.Cluster = nil
		client.IsCluster = false
		return
	}

	if !client.IsCluster && args.Cluster {
		log.Printf(mildWarning+" Cluster mode enabled but %s is not a cluster", args.RedisAddress)
		return
	}
}

func failOnErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
}
