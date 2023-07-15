package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/fatih/color"
)

var warningWord = color.New(color.FgRed).Sprint("WARNING!")
var dangerousWord = color.New(color.FgRed).Sprint("DANGEROUS")

type Args struct {
	Count        *CountArgs  `arg:"subcommand:count"`
	Print        *PrintArgs  `arg:"subcommand:print"`
	Delete       *DeleteArgs `arg:"subcommand:delete"`
	RedisAddress string      `arg:"-a,--address,required" help:"redis server address. Eg: localhost:6379"`
	Cluster      bool        `arg:"-c,--cluster" help:"connect in cluster mode"`
}

type CountArgs struct {
	ScanArgs
}

type PrintArgs struct {
	ScanArgs
}

type DeleteArgs struct {
	ScanArgs
	DeletionBatchSize int  `arg:"--delete-batch" default:"50" help:"delete this amount of keys per command"`
	UnsafeNoCount     bool `arg:"--unsafe-no-count" default:"false" help:"WARNING: This option runs faster but is considerably dangerous. By default keys are counted first, deletion is confirmed by the user and then deletion starts. With this option we skip key counting. There is only confirmation to move forward. Use in situations where you are absolutely sure of your inputs and you already have a good sense of the amount of keys that will be deleted beforehand"`
}

type ScanArgs struct {
	Pattern   string        `arg:"-p,--pattern,required" help:"Which pattern to search for. The pattern format is the same documented in the SCAN command"`
	BatchSize int64         `arg:"-b,--batch" default:"1000" help:"How many keys to scan at a time. The higher the number, higher is the perfomance but also higher the load in redis"`
	Wait      time.Duration `arg:"-w,--wait" default:"0s" help:"Wait this amount of time between batches"`
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

	if client.IsCluster && !args.Cluster {
		log.Printf(warningWord + " Connecting to a node that is a member of a cluster, but cluster mode (-c|--cluster) is NOT enabled. Commands will either fail or be local to this particular node. Waiting 5 seconds before continuing")
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
		}
		client.Cluster = nil
		client.IsCluster = false
	}

	switch {
	case args.Count != nil:
		err = Count(ctx, parser, client, args)
	case args.Print != nil:
		err = Print(ctx, parser, client, args)
	case args.Delete != nil:
		err = Delete(ctx, parser, client, args)
	}

	failOnErr(err)
}

func Count(ctx context.Context, parser *arg.Parser, client UnifiedClient, args Args) error {
	var count int64
	callback := func(_ context.Context, _ *Client, keys []string) error {
		atomic.AddInt64(&count, int64(len(keys)))
		return nil
	}

	if err := scanCmd(ctx, parser, client, args.Count.ScanArgs, callback); err != nil {
		return err
	}

	fmt.Println(count)
	return nil
}

func Print(ctx context.Context, parser *arg.Parser, client UnifiedClient, args Args) error {
	callback := func(_ context.Context, _ *Client, keys []string) error {
		for _, key := range keys {
			fmt.Println(key)
		}
		return nil
	}

	return scanCmd(ctx, parser, client, args.Print.ScanArgs, callback)
}

func Delete(ctx context.Context, parser *arg.Parser, client UnifiedClient, args Args) error {
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

	deletionLogFile, err := os.CreateTemp("", fmt.Sprintf("deleted-redis-keys-%s", args.RedisAddress))
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

	if err := scanCmd(ctx, parser, client, args.Delete.ScanArgs, callback); err != nil {
		return err
	}

	msg := fmt.Sprintf(""+
		warningWord+" Deleting an estimate of %d keys with pattern %s in %s.\n"+
		"Check %s for selected Keys.\n"+
		"Keys being deleted will be logged to %s",
		count, args.Delete.Pattern, args.RedisAddress, listedKeysFile.Name(), deletionLogFile.Name(),
	)
	if err := userConfirm(ctx, msg, args.RedisAddress, 5*time.Second); err != nil {
		return err
	}

	return scanCmd(ctx, parser, client, args.Delete.ScanArgs, deleteCallback(args.Delete.DeletionBatchSize, deletionLogFile))
}

func deleteWithoutCount(ctx context.Context, parser *arg.Parser, client UnifiedClient, args Args) error {
	deletionLogFile, err := os.CreateTemp("", fmt.Sprintf("deleted-redis-keys-%s-", args.RedisAddress))
	if err != nil {
		return err
	}
	defer deletionLogFile.Close()

	msg := fmt.Sprintf(""+
		warningWord+" --unsafe-no-count passed. "+
		"Skipping initial key counting. "+
		"Keys with pattern %s will be deleted as they are found in %s. "+
		"This is a faster but "+dangerousWord+" option.\n"+
		"Keys being deleted will be logged to %s",
		args.Delete.Pattern, args.RedisAddress, deletionLogFile.Name(),
	)
	if err := userConfirm(ctx, msg, args.RedisAddress, 5*time.Second); err != nil {
		return err
	}

	return scanCmd(ctx, parser, client, args.Delete.ScanArgs, deleteCallback(args.Delete.DeletionBatchSize, deletionLogFile))
}

func deleteCallback(batchSize int, logFile *os.File) func(ctx context.Context, client *Client, keys []string) error {
	return func(ctx context.Context, client *Client, keys []string) error {
		mutex := sync.Mutex{}
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

			line := strings.Builder{}
			line.WriteString(strconv.FormatInt(deleted, 10))
			line.WriteString(" ")
			line.WriteString(strings.Join(batch, " "))
			line.WriteString("\n")

			mutex.Lock()
			if _, err = logFile.WriteString(line.String()); err != nil {
				mutex.Unlock()
				return err
			}
			mutex.Unlock()
		}
		return nil
	}
}

func scanCmd(ctx context.Context, parser *arg.Parser, client UnifiedClient, args ScanArgs, callback func(ctx context.Context, client *Client, keys []string) error) error {
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
			return IterateKeys(ctx, client, args.Pattern, args.BatchSize, callback)
		})
	} else {
		err = IterateKeys(ctx, client.Single, args.Pattern, args.BatchSize, callback)
	}

	return err
}

var errUserAborted = errors.New("user aborted")

func userConfirm(ctx context.Context, message string, confirmationText string, thinkTime time.Duration) error {
	if noConfirm, _ := os.LookupEnv("UNSAFE_NO_CONFIRM"); noConfirm == "true" {
		return nil
	}

	fmt.Println(message)

	if thinkTime > 0 {
		fmt.Printf("Waiting %v before asking for confirmation\n", thinkTime)
		select {
		case <-time.After(thinkTime):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	var question string
	if confirmationText == "" || confirmationText == "y" {
		question = "Confirm? [y/n] "
		confirmationText = "y"
	} else {
		question = fmt.Sprintf("Type %s to confirm: ", confirmationText)
	}

	fmt.Fprintf(os.Stderr, question)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		if scanner.Text() != confirmationText {
			return errUserAborted
		}
		return nil
	}
	if scanner.Err() != nil {
		return scanner.Err()
	}
	return errUserAborted
}

func failOnErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
}
