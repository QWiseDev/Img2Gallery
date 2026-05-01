package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/app"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
)

func main() {
	cfg := config.Load()
	if err := app.EnsureStorage(cfg); err != nil {
		log.Fatalf("storage init failed: %v", err)
	}
	handler, database, err := app.New(cfg)
	if err != nil {
		log.Fatalf("app init failed: %v", err)
	}
	defer database.Close()

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("server listening on %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server shutdown failed: %v", err)
	}
}
