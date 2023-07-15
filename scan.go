package main

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/alexflint/go-arg"
)

type ScanArgs struct {
	Pattern   string        `arg:"-p,--pattern,required" help:"Which pattern to search for. The pattern format is the same documented in the SCAN command"`
	BatchSize int64         `arg:"-b,--batch" default:"1000" help:"How many keys to scan at a time. The higher the number, higher is the perfomance but also higher the load in redis"`
	Wait      time.Duration `arg:"-w,--wait" default:"0s" help:"Wait this amount of time between batches"`
}

func ScanCmd(ctx context.Context, parser *arg.Parser, client UnifiedClient, args ScanArgs, callback func(ctx context.Context, client *Client, keys []string) error) error {
	if args.Pattern == "" {
		parser.Fail("--pattern must be set to a value. To count all keys use --pattern '*'")
	}

	if args.Wait > 0 {
		originalCallback := callback
		callback = func(ctx context.Context, client *Client, keys []string) error {
			if err := originalCallback(ctx, client, keys); err != nil {
				return err
			}
			select {
			case <-time.After(args.Wait):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	var err error
	if client.IsCluster {
		err = client.Cluster.ForEachShard(ctx, func(ctx context.Context, client *Client) error {
			return ScanKeys(ctx, client, args.Pattern, args.BatchSize, callback)
		})
	} else {
		err = ScanKeys(ctx, client.Single, args.Pattern, args.BatchSize, callback)
	}

	return err
}

func ScanKeys(ctx context.Context, client *Client, pattern string, blockSize int64, callbackFn func(context.Context, *Client, []string) error) error {
	start := time.Now()
	var processedKeys int64
	var cursor uint64

	logProgress := func() {
		timeTaken := time.Since(start)
		processedKeys := atomic.LoadInt64(&processedKeys)
		speedSeconds := float64(processedKeys) / timeTaken.Seconds()
		log.Printf("[%s] processed %d keys (~%.2f keys/s)", client.Addr, processedKeys, speedSeconds)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		for {
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
			logProgress()
		}
	}()

	for {
		cmd := client.Scan(ctx, cursor, pattern, blockSize)
		keys, nextCursor, err := cmd.Result()
		if err != nil {
			return err
		}

		if len(keys) == 0 {
			break
		}

		atomic.AddInt64(&processedKeys, int64(len(keys)))
		if err := callbackFn(ctx, client, keys); err != nil {
			return err
		}

		if nextCursor == 0 {
			break
		}

		cursor = nextCursor
	}

	logProgress()
	return nil
}
