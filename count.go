package main

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/alexflint/go-arg"
)

type CountArgs struct {
	ScanArgs
}

func CountCmd(ctx context.Context, parser *arg.Parser, client UnifiedClient, args Args) error {
	var count int64
	callback := func(_ context.Context, _ *Client, keys []string) error {
		atomic.AddInt64(&count, int64(len(keys)))
		return nil
	}

	if err := ScanCmd(ctx, parser, client, args.Count.ScanArgs, callback); err != nil {
		return err
	}

	fmt.Println(count)
	return nil
}
