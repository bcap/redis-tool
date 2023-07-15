package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/bcap/humanize"
)

type DeleteArgs struct {
	ScanArgs
	DeletionBatchSize int  `arg:"--delete-batch" default:"50" help:"delete this amount of keys per command"`
	UnsafeNoCount     bool `arg:"--unsafe-no-count" default:"false" help:"WARNING: This option runs faster but is considerably dangerous. By default keys are counted first, deletion is confirmed by the user and then deletion starts. With this option we skip key counting. There is only confirmation to move forward. Use in situations where you are absolutely sure of your inputs and you already have a good sense of the amount of keys that will be deleted beforehand"`
}

func DeleteCmd(ctx context.Context, parser *arg.Parser, client UnifiedClient, args Args) error {
	if args.Delete.UnsafeNoCount {
		return deleteWithoutCount(ctx, parser, client, args)
	} else {
		return deleteWithCount(ctx, parser, client, args)
	}
}

func deleteWithCount(ctx context.Context, parser *arg.Parser, client UnifiedClient, args Args) error {
	listedKeysFile, err := os.CreateTemp("", fmt.Sprintf("listed-redis-keys-for-deletion-%s-", args.RedisAddress))
	if err != nil {
		return err
	}
	defer listedKeysFile.Close()

	deletionLogFile, err := os.CreateTemp("", fmt.Sprintf("deleted-redis-keys-%s-", args.RedisAddress))
	if err != nil {
		return err
	}
	defer deletionLogFile.Close()

	log.Print("Counting keys for deletion")

	var count int64
	mutex := sync.Mutex{}
	callback := func(_ context.Context, _ *Client, keys []string) error {
		mutex.Lock()
		defer mutex.Unlock()
		count += int64(len(keys))
		for _, key := range keys {
			if _, err := listedKeysFile.WriteString(key + "\n"); err != nil {
				return err
			}
		}
		return nil
	}

	if err := ScanCmd(ctx, parser, client, args.Delete.ScanArgs, callback); err != nil {
		return err
	}

	if count == 0 {
		log.Printf("No keys with pattern %s were found", args.Delete.Pattern)
		return nil
	}

	msg := fmt.Sprintf(""+
		"%s Deleting an estimate of %d keys with pattern %s in %s.\n"+
		"Check %s for selected Keys.\n"+
		"Keys being deleted will be logged to %s",
		mildWarning, count, args.Delete.Pattern, args.RedisAddress,
		listedKeysFile.Name(), deletionLogFile.Name(),
	)
	if err := UserConfirm(ctx, msg, args.RedisAddress, 5*time.Second); err != nil {
		return err
	}

	return delete(ctx, parser, client, args, deletionLogFile)
}

func deleteWithoutCount(ctx context.Context, parser *arg.Parser, client UnifiedClient, args Args) error {
	deletionLogFile, err := os.CreateTemp("", fmt.Sprintf("deleted-redis-keys-%s-", args.RedisAddress))
	if err != nil {
		return err
	}
	defer deletionLogFile.Close()

	msg := fmt.Sprintf(""+
		"%s --unsafe-no-count passed. "+
		"Skipping initial key counting. "+
		"Keys with pattern %s will be deleted as they are found in %s. "+
		"This is a faster but "+dangerous+" option.\n"+
		"Keys being deleted will be logged to %s",
		severeWarning, args.Delete.Pattern, args.RedisAddress, deletionLogFile.Name(),
	)
	if err := UserConfirm(ctx, msg, args.RedisAddress, 5*time.Second); err != nil {
		return err
	}

	return delete(ctx, parser, client, args, deletionLogFile)
}

func delete(ctx context.Context, parser *arg.Parser, client UnifiedClient, args Args, logFile *os.File) error {
	var totalDeleted int64
	batchSize := args.Delete.DeletionBatchSize
	mutex := sync.Mutex{}
	callback := func(ctx context.Context, client *Client, keys []string) error {
		for idx := 0; idx < len(keys); idx += batchSize {
			endIdx := idx + batchSize
			if endIdx > len(keys) {
				endIdx = len(keys)
			}
			batch := keys[idx:endIdx]
			deleted, err := client.Del(ctx, batch...).Result()
			if err != nil {
				return err
			}

			atomic.AddInt64(&totalDeleted, deleted)

			line := strings.Builder{}
			line.WriteString(strconv.FormatInt(deleted, 10))
			for _, key := range batch {
				line.WriteString(" ")
				line.WriteString(key)
			}
			line.WriteString("\n")
			lineStr := line.String()

			mutex.Lock()
			_, err = logFile.WriteString(lineStr)
			mutex.Unlock()

			if err != nil {
				return err
			}
		}
		return nil
	}

	start := time.Now()

	err := ScanCmd(ctx, parser, client, args.Delete.ScanArgs, callback)
	if err != nil {
		return err
	}

	log.Printf("Deleted %d keys in %s", totalDeleted, humanize.Duration(time.Since(start)))
	return nil
}
