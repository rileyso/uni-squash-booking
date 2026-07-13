package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/rileyso/uni-squash-booking/internal/app"
	"github.com/rileyso/uni-squash-booking/internal/config"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
	"github.com/rileyso/uni-squash-booking/internal/web"
)

func main() {
	configuration, err := config.Load(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := sqlite.Open(ctx, configuration.DatabasePath, configuration.RecoveryGeneration)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	application, err := app.New(configuration, store)
	if err != nil {
		log.Fatal(err)
	}
	adapter, err := web.New(application)
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: configuration.Address, Handler: adapter.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown_error=%q", err)
		}
	}()
	log.Printf("event=server_start address=%s environment=%s synthetic=%t", configuration.Address, configuration.Environment, configuration.Synthetic)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
