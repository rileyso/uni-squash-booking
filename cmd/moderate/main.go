package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/rileyso/uni-squash-booking/internal/app"
	"github.com/rileyso/uni-squash-booking/internal/config"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
	}
	action := os.Args[1]
	flags := flag.NewFlagSet(action, flag.ExitOnError)
	identifier := flags.String("identifier", "", "exact full username or four-digit player code")
	confirm := flags.String("confirm", "", "repeat the exact full username for permanent deletion")
	backup := flags.String("backup", "", "new file path for the verified pre-action backup")
	if err := flags.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}
	configuration, err := config.Load(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	store, err := sqlite.Open(ctx, configuration.DatabasePath, configuration.RecoveryGeneration)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	service, err := app.New(configuration, store)
	if err != nil {
		log.Fatal(err)
	}
	var account sqlite.ModerationAccount
	switch action {
	case "suspend":
		account, err = service.SuspendAccount(ctx, *identifier)
	case "reinstate":
		account, err = service.ReinstateAccount(ctx, *identifier)
	case "delete":
		account, err = service.PermanentlyDeleteAccount(ctx, *identifier, *confirm, *backup)
	default:
		usage()
	}
	if err != nil {
		log.Printf("security_event=moderation action=%s outcome=failure at_utc=%s", action, time.Now().UTC().Format(time.RFC3339))
		log.Fatal(err)
	}
	log.Printf("security_event=moderation action=%s target_account_id=%d outcome=success at_utc=%s", action, account.ID, time.Now().UTC().Format(time.RFC3339))
	fmt.Printf("%s succeeded for account ID %d\n", action, account.ID)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: moderate <suspend|reinstate|delete> --identifier <exact username or player code> [--confirm <exact username> --backup <new snapshot path>]")
	os.Exit(2)
}
