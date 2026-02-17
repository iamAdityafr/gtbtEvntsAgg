package utils

import (
	"context"
	"encoding/json"
	"githubEventsAggregator/internal/events"
	"githubEventsAggregator/internal/logger"
	"githubEventsAggregator/internal/model"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	FETCH_INTERVAL = 3 * time.Minute
)

var (
	fetchMu sync.Mutex
)

func Fetchit(url string, repo *events.Repo) error {
	logger.Lg.Info("api_fetch_flight", zap.String("url", url))

	res, err := http.Get(url)
	if err != nil {
		logger.Lg.Error("api_fetch_error", zap.Error(err))
		return err
	}

	logger.Lg.Info("api_fetch_done",
		zap.String("url", url),
		zap.Int("status", res.StatusCode),
		zap.Int("content_length", int(res.ContentLength)),
	)
	var oy []model.Event
	err = json.NewDecoder(res.Body).Decode(&oy)
	if err != nil {
		return err
	}
	for _, e := range oy[:5] {
		// fmt.Println(e.Type, e.Repo.Name, e.Actor.Login)
		err := repo.Save(&e)
		if err != nil {
			return err
		}
	}
	res.Body.Close()
	return nil
}
func FetchitWorker(ctx context.Context, wg *sync.WaitGroup, r *events.Repo) {
	defer wg.Done()
	ticker := time.NewTicker(FETCH_INTERVAL)
	defer ticker.Stop()
	fetchMu.Lock()
	if err := Fetchit("https://api.github.com/events", r); err != nil {
		logger.Lg.Error("flight fetch error", zap.Error(err))
	}
	fetchMu.Unlock()
	for {
		select {
		case <-ctx.Done():
			logger.Lg.Info("Ticker stopping")
			return
		case <-ticker.C:
			fetchMu.Lock()
			if err := Fetchit("https://api.github.com/events", r); err != nil {
				logger.Lg.Error("fetch error", zap.Error(err))
				os.Exit(1)
			}
			fetchMu.Unlock()
		}
	}
}
