package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

var ErrUserAborted = errors.New("user aborted")

func UserConfirm(ctx context.Context, message string, confirmationText string, thinkTime time.Duration) error {
	if noConfirm, _ := os.LookupEnv("UNSAFE_NO_CONFIRM"); noConfirm == "true" {
		return nil
	}

	for _, line := range strings.Split(message, "\n") {
		log.Print(line)
	}

	if thinkTime > 0 {
		log.Printf("Waiting %v before asking for user confirmation\n", thinkTime)
		select {
		case <-time.After(thinkTime):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if confirmationText == "" {
		confirmationText = "y"
	}

	fmt.Fprintf(os.Stderr, "Type %s to confirm: ", confirmationText)

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		if scanner.Text() != confirmationText {
			return ErrUserAborted
		}
		return nil
	}
	if scanner.Err() != nil {
		return scanner.Err()
	}
	return ErrUserAborted
}
