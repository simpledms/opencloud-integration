package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("integration stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	store := newTokenStore()
	cloud, err := newOpenCloud(c)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{
		Addr: c.ListenAddr, Handler: newService(c, store, cloud),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 31 * time.Minute, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 32 * 1024,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				store.purge()
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := server.Shutdown(shutdownCtx); err != nil {
					slog.Warn("shutdown interrupted active downloads")
				}
				return
			}
		}
	}()
	slog.Info("SimpleDMS integration listening", "address", c.ListenAddr)
	err = server.ListenAndServe()
	stop()
	<-done
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
