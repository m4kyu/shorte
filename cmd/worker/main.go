package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shorte/internal/app"
)

func main() {
	cfg, err := app.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	w, err := app.NewWorker(cfg)
	if err != nil {
		log.Fatalf("init worker: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := w.Run(ctx); err != nil {
			log.Fatalf("worker run: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	cancel()
	time.Sleep(500 * time.Millisecond)
}
