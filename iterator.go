package main

import (
	"context"
	"log"
	"sync/atomic"
	"time"
)

func IterateKeys(ctx context.Context, client *Client, pattern string, blockSize int64, callbackFn func(context.Context, *Client, []string) error) error {
	start := time.Now()
	var processedKeys int64
	var cursor uint64

	infoCmd := client.Info(ctx)
	if infoCmd.Err() != nil {
		return infoCmd.Err()
	}

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
