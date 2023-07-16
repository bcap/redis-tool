package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"

	"github.com/alexflint/go-arg"
	"golang.org/x/sync/errgroup"
)

type DumpArgs struct {
	ScanArgs
	InParallel int `arg:"--parallel" default:"1" help:"how many keys to dump in parallel once they are scanned"`
}

func (*DumpArgs) Description() string {
	return "Dumps keys and their respective values"
}

func DumpCmd(ctx context.Context, parser *arg.Parser, client UnifiedClient, args Args) error {
	type chanEntry struct {
		keys   []string
		client *Client
	}

	parallel := args.Dump.InParallel
	if parallel <= 0 {
		parallel = 1
	}

	channel := make(chan chanEntry)
	group, ctx := errgroup.WithContext(ctx)

	printMutex := sync.Mutex{}

	for i := 0; i < parallel; i++ {
		group.Go(func() error {
			for {
				select {
				case entry, ok := <-channel:
					if !ok {
						return nil
					}
					if err := DumpKeys(ctx, entry.client, entry.keys, os.Stdout, &printMutex); err != nil {
						return err
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}

	callback := func(ctx context.Context, client *Client, keys []string) error {
		select {
		case channel <- chanEntry{keys: keys, client: client}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	group.Go(func() error {
		defer close(channel)
		return ScanCmd(ctx, parser, client, args.Dump.ScanArgs, callback)
	})

	return group.Wait()
}

func DumpKeys(ctx context.Context, client *Client, keys []string, writer io.Writer, writeMutex *sync.Mutex) error {
	stringKeys := []string{}
	listKeys := []string{}
	setKeys := []string{}
	zsetKeys := []string{}
	hashKeys := []string{}
	streamKeys := []string{}
	unknownTypeKeys := []string{}

	for _, key := range keys {
		keyType, err := client.Type(ctx, key).Result()
		if err != nil {
			return err
		}
		// https://redis.io/commands/type/
		// types: string, list, set, zset, hash and stream.
		switch keyType {
		case "string":
			stringKeys = append(stringKeys, key)
		case "list":
			listKeys = append(listKeys, key)
		case "set":
			setKeys = append(setKeys, key)
		case "zset":
			zsetKeys = append(zsetKeys, key)
		case "hash":
			hashKeys = append(hashKeys, key)
		case "stream":
			streamKeys = append(streamKeys, key)
		default:
			unknownTypeKeys = append(unknownTypeKeys, key)
		}
	}

	marshaller := json.NewEncoder(writer)

	if len(stringKeys) > 0 {
		values, err := client.MGet(ctx, stringKeys...).Result()
		if err != nil {
			return err
		}

		writeMutex.Lock()
		for idx, key := range stringKeys {
			value := values[idx]
			data := map[string]any{
				"key":   key,
				"type":  "string",
				"value": value,
			}
			if err := marshaller.Encode(data); err != nil {
				writeMutex.Unlock()
				return err
			}
		}
		writeMutex.Unlock()
	}

	for _, key := range listKeys {
		list, err := client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			return err
		}

		data := map[string]any{
			"key":   key,
			"type":  "list",
			"value": list,
		}
		writeMutex.Lock()
		if err := marshaller.Encode(data); err != nil {
			writeMutex.Unlock()
			return err
		}
		writeMutex.Unlock()
	}

	for _, key := range setKeys {
		var cursor uint64
		values := []string{}
		for {
			valuesBatch, cursor, err := client.SScan(ctx, key, cursor, "", 1000).Result()
			if err != nil {
				return err
			}
			values = append(values, valuesBatch...)
			if cursor == 0 {
				break
			}
		}

		data := map[string]any{
			"key":   key,
			"type":  "set",
			"value": values,
		}
		writeMutex.Lock()
		if err := marshaller.Encode(data); err != nil {
			writeMutex.Unlock()
			return err
		}
		writeMutex.Unlock()
	}

	for _, key := range zsetKeys {
		zvalues, err := client.ZRangeWithScores(ctx, key, 0, -1).Result()
		if err != nil {
			return err
		}

		values := make([]map[string]any, len(zvalues))
		for idx, zvalue := range zvalues {
			values[idx] = map[string]any{
				"score": zvalue.Score,
				"value": zvalue.Member,
			}
		}

		data := map[string]any{
			"key":   key,
			"type":  "zset",
			"value": values,
		}
		writeMutex.Lock()
		if err := marshaller.Encode(data); err != nil {
			writeMutex.Unlock()
			return err
		}
		writeMutex.Unlock()
	}

	for _, key := range hashKeys {
		values, err := client.HGetAll(ctx, key).Result()
		if err != nil {
			return err
		}
		data := map[string]any{
			"key":   key,
			"type":  "hash",
			"value": values,
		}
		writeMutex.Lock()
		if err := marshaller.Encode(data); err != nil {
			writeMutex.Unlock()
			return err
		}
		writeMutex.Unlock()
	}

	writeMutex.Lock()
	for _, key := range streamKeys {
		data := map[string]any{
			"key":   key,
			"type":  "stream",
			"value": nil,
		}
		if err := marshaller.Encode(data); err != nil {
			writeMutex.Unlock()
			return err
		}
	}
	writeMutex.Unlock()

	writeMutex.Lock()
	for _, key := range unknownTypeKeys {
		data := map[string]any{
			"key":   key,
			"type":  "unknown",
			"value": nil,
		}
		if err := marshaller.Encode(data); err != nil {
			writeMutex.Unlock()
			return err
		}
	}
	writeMutex.Unlock()

	return nil
}
