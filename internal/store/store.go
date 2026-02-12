package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"githubEventsAggregator/internal/logger"
	"githubEventsAggregator/internal/model"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	Db  *sql.DB
	Rdb *redis.Client
	Ctx = context.Background()
)

func init() {

	var err error
	Db, err = sql.Open("sqlite3", "./events.db")
	if err != nil {
		logger.Lg.Error("sql open", zap.Error(err))
		os.Exit(1)
	}

	Rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
}

func CreateTable(db *sql.DB) (sql.Result, error) {
	sqlstmt := `CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		type TEXT,
		repo TEXT,
		actor TEXT,
		created_at TEXT
);`
	return db.Exec(sqlstmt)
}

func FetchDB(db *sql.DB) error {
	rows, err := db.Query("SELECT id, name, type FROM events")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, name, typ string
		err = rows.Scan(&id, &name, &typ)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ID: %s, Name: %s, Age: %s\n", id, name, typ)
	}
	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}
	return nil
}

func FetchRedis() {
	var cursor uint64
	var events []model.Event

	for {
		// match "event:*"
		keys, nextCursor, err := Rdb.Scan(Ctx, cursor, "event:*", 100).Result()
		if err != nil {
			log.Fatal(err)
		}

		for _, key := range keys {
			val, err := Rdb.Get(Ctx, key).Result()
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
}
