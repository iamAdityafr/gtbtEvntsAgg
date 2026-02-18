package events

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"githubEventsAggregator/internal/logger"
	"githubEventsAggregator/internal/model"
	"githubEventsAggregator/internal/store"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RepoInterface interface {
	Save(e *model.Event) error
	GetEventById(id string) (*model.Event, error)
	GetAllEvents() ([]model.Event, error)
	GetAggJson() ([]byte, bool, error)
	SetAggJson(data []byte, ttlseconds int) error
}

type Repo struct {
	db  *sql.DB
	Rdb *redis.Client
}

func NewRepo(db *sql.DB, Rdb *redis.Client) *Repo {
	return &Repo{db: db, Rdb: Rdb}
}

func (r *Repo) SetAggJson(data []byte, ttlseconds int) error {
	return r.Rdb.Set(store.Ctx, "events:agg", data, time.Duration(ttlseconds)*time.Second).Err()
}

func (r *Repo) GetAggJson() ([]byte, bool, error) {
	val, err := r.Rdb.Get(store.Ctx, "events:agg").Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return []byte(val), true, nil
}

func (r *Repo) GetAllEvents() ([]model.Event, error) {
	var cursor uint64
	var events []model.Event

	for {
		keys, nextCursor, err := store.Rdb.Scan(store.Ctx, cursor, "event:*", 100).Result()
		if err != nil {
			logger.Lg.Error("redis scan", zap.Error(err))
			os.Exit(1)
		}

		for _, key := range keys {
			val, err := store.Rdb.Get(store.Ctx, key).Result()
			if err != nil {
				continue
			}
			var event model.Event
			if err := json.Unmarshal([]byte(val), &event); err != nil {
				continue
			}
			events = append(events, event)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	fmt.Println("Total events in memory:", len(events))
	return events, nil
}
func (r *Repo) GetEventById(id string) (*model.Event, error) {
	cachekey := "event:" + id

	val, err := store.Rdb.Get(store.Ctx, cachekey).Result()
	if err == nil {
		var e model.Event
		json.Unmarshal([]byte(val), &e)
		return &e, nil
	}
	row := store.Db.QueryRow(`
			SELECT id, type, actor, repo, created_at
			FROM events WHERE id = ?`, id)

	var event model.Event
	var actorStr, repoStr string
	err = row.Scan(
		&event.ID,
		&event.Type,
		&repoStr,
		&event.CreatedAt,
		&actorStr,
	)
	event.Actor.Login = actorStr
	event.Repo.Name = repoStr
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(event)
	store.Rdb.Set(store.Ctx, cachekey, data, 0)

	return &event, nil
}
func (r *Repo) Save(e *model.Event) error {
	_, err := store.Db.Exec(`
		INSERT OR REPLACE INTO events
		(id, type, actor, repo, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		e.ID, e.Type, e.Actor.Login, e.Repo.Name, e.CreatedAt,
	)
	if err != nil {
		return err
	}
	store.Db.Exec(`
		DELETE FROM events
		WHERE id NOT IN (
			SELECT id FROM events
			ORDER BY created_at DESC
			LIMIT 30
		)
	`)
	data, _ := json.Marshal(e)
	store.Rdb.Set(store.Ctx, "event:"+e.ID, data, 0)

	store.Rdb.LPush(store.Ctx, "event_cache_keys", e.ID)
	store.Rdb.LTrim(store.Ctx, "event_cache_keys", 0, 9)

	//evict older keys
	keys, _ := store.Rdb.LRange(store.Ctx, "event_cache_keys", 10, -1).Result()
	for _, k := range keys {
		store.Rdb.Del(store.Ctx, "event:"+k)
	}

	return nil
}
