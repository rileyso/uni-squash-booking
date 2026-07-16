package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ExitOnError)
	database := flags.String("database", "", "database path")
	source := flags.String("source", "", "verified snapshot path")
	destination := flags.String("destination", "", "new restore or backup path")
	generation := flags.String("generation", "", "expected recovery generation")
	_ = flags.Parse(os.Args[2:])
	ctx := context.Background()
	var err error
	switch command {
	case "backup":
		var store *sqlite.Store
		store, err = sqlite.Open(ctx, *database, *generation)
		if err == nil {
			defer store.Close()
			err = store.Backup(ctx, *destination)
		}
	case "restore":
		err = sqlite.Restore(ctx, *source, *destination, *generation)
	case "scrub-identities":
		err = sqlite.ScrubRestoredIdentities(ctx, *database, *generation)
	case "clear-lockdown":
		err = sqlite.ClearRecoveryLockdown(ctx, *database, *generation)
	default:
		usage()
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("recovery_operation=%s status=complete\n", command)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: recovery <backup|restore|scrub-identities|clear-lockdown> [options]")
	os.Exit(2)
}
