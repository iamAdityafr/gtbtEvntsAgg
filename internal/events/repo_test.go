// internal/events/repo_test.go
package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"githubEventsAggregator/internal/model"
	"githubEventsAggregator/internal/store"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	_ "modernc.org/sqlite"
)

type RepoTestSuite struct {
	suite.Suite
	redisContainer *tcredis.RedisContainer
	redisClient    *redis.Client
	sqlDB          *sql.DB
	repo           *Repo
	ctx            context.Context
}

func (s *RepoTestSuite) SetupSuite() {
	s.ctx = context.Background()

	redisC, err := tcredis.Run(s.ctx, "redis:7-alpine")
	if err != nil {
		s.T().Fatalf("failed to start redis container: %v", err)
	}
	s.redisContainer = redisC

	host, err := redisC.Host(s.ctx)
	if err != nil {
		s.T().Fatal(err)
	}

	port, err := redisC.MappedPort(s.ctx, "6379")
	if err != nil {
		s.T().Fatal(err)
	}

	s.redisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})

	s.sqlDB, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		s.T().Fatalf("failed to open sqlite: %v", err)
	}

	s.setupSchema()
	store.Db = s.sqlDB
	store.Rdb = s.redisClient
	store.Ctx = s.ctx

	s.repo = NewRepo(s.sqlDB, s.redisClient)
}

func (s *RepoTestSuite) setupSchema() {
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		type TEXT,
		actor TEXT,
		repo TEXT,
		created_at TEXT
	);
	`
	_, err := s.sqlDB.Exec(schema)
	if err != nil {
		s.T().Fatalf("failed to create schema: %v", err)
	}
}

func (s *RepoTestSuite) TearDownSuite() {
	if s.redisContainer != nil {
		if err := s.redisContainer.Terminate(s.ctx); err != nil {
			s.T().Logf("failed to terminate redis container: %v", err)
		}
	}
	if s.sqlDB != nil {
		s.sqlDB.Close()
	}
}

func (s *RepoTestSuite) SetupTest() {
	s.redisClient.FlushAll(s.ctx)
	s.sqlDB.Exec("DELETE FROM events")
}

func TestRepoTestSuite(t *testing.T) {
	suite.Run(t, new(RepoTestSuite))
}

func (s *RepoTestSuite) TestSaveAndGetByID() {
	event := &model.Event{
		ID:        "123",
		Type:      "PushEvent",
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	event.Actor.Login = "testuser"
	event.Repo.Name = "testrepo"

	err := s.repo.Save(event)
	s.NoError(err)

	cached, err := s.repo.GetEventById("123")
	s.NoError(err)
	s.Equal("123", cached.ID)
	s.Equal("PushEvent", cached.Type)
	s.Equal("testuser", cached.Actor.Login)
	s.Equal("testrepo", cached.Repo.Name)

	val, err := s.redisClient.Get(s.ctx, "event:123").Result()
	s.NoError(err)
	s.Contains(val, "PushEvent")
}

func (s *RepoTestSuite) TestGetAllEvents() {
	events := []*model.Event{
		{ID: "1", Type: "PushEvent", CreatedAt: time.Now().Format(time.RFC3339)},
		{ID: "2", Type: "PullRequestEvent", CreatedAt: time.Now().Format(time.RFC3339)},
	}
	for _, e := range events {
		s.repo.Save(e)
	}

	all, err := s.repo.GetAllEvents()
	s.NoError(err)
	s.Len(all, 2)
}

func (s *RepoTestSuite) TestSave_Eviction() {
	for i := 1; i <= 12; i++ {
		e := &model.Event{
			ID:        fmt.Sprintf("event%d", i),
			Type:      "PushEvent",
			CreatedAt: time.Now().Format(time.RFC3339),
		}
		err := s.repo.Save(e)
		s.NoError(err)
	}

	keys, err := s.redisClient.LRange(s.ctx, "event_cache_keys", 0, -1).Result()
	s.NoError(err)
	s.Len(keys, 10)
	s.Equal("event12", keys[0]) // mst recent
	s.Equal("event3", keys[9])  // oldest one
}

func (s *RepoTestSuite) TestFullWorkflow() {
	event := &model.Event{
		ID:        "testid",
		Type:      "testEvent",
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	event.Actor.Login = "testlogin"
	event.Repo.Name = "testrepo"

	err := s.repo.Save(event)
	s.NoError(err)

	byID, err := s.repo.GetEventById("testid")
	s.NoError(err)
	s.Equal("testEvent", byID.Type)

	all, err := s.repo.GetAllEvents()
	s.NoError(err)
	s.Len(all, 1)

	jsonData, _ := json.Marshal(all)
	err = s.repo.SetAggJson(jsonData, 300)
	s.NoError(err)

	cached, hit, err := s.repo.GetAggJson()
	s.NoError(err)
	s.True(hit)
	s.JSONEq(string(jsonData), string(cached))
}
