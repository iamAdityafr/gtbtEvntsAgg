package main

import (
	"context"
	"githubEventsAggregator/internal/events"
	"githubEventsAggregator/internal/handlers"
	"githubEventsAggregator/internal/logger"
	"githubEventsAggregator/internal/middleware"
	"githubEventsAggregator/internal/store"
	"githubEventsAggregator/internal/utils"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func main() {
	logger.InitLogger()
	store.CreateTable(store.Db) //ignores if already exists btw
	defer logger.Lg.Sync()
	r := events.NewRepo(store.Db, store.Rdb)
	handler := events.NewService(r)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go utils.FetchitWorker(ctx, wg, r)

	app := fiber.New()
	app.Use(middleware.RequestLogger())
	h := handlers.NewHTTP(handler)

	// endpoints
	app.Get("/events/:id", h.GetEventById)
	app.Get("/events", h.GetEvents)

	go func() {
		if err := app.Listen(":3000"); err != nil {
			logger.Lg.Info("Server stopped", zap.Error(err))
		}
	}()

	GracefulShutdown(app, cancel, wg)
	logger.Lg.Info("Shutdown complete")
}

func GracefulShutdown(app *fiber.App, cancel context.CancelFunc, wg *sync.WaitGroup) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	<-sigchan
	logger.Lg.Info("Shutdown sig rcv")
	cancel()
	if err := store.Db.Close(); err != nil {
		logger.Lg.Error("db close error", zap.Error(err))
	}
	if err := app.Shutdown(); err != nil {
		logger.Lg.Error("Server shutdown error", zap.Error(err))
	}
	wg.Wait()
}
