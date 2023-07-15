package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/alexflint/go-arg"
)

type PrintArgs struct {
	ScanArgs
}

func PrintCmd(ctx context.Context, parser *arg.Parser, client UnifiedClient, args Args) error {
	mutex := sync.Mutex{}
	callback := func(_ context.Context, _ *Client, keys []string) error {
		mutex.Lock()
		for _, key := range keys {
			fmt.Println(key)
		}
		mutex.Unlock()
		return nil
	}

	return ScanCmd(ctx, parser, client, args.Print.ScanArgs, callback)
}
