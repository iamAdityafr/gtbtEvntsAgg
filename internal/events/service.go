package events

import (
	"encoding/json"
	"githubEventsAggregator/internal/logger"
	"githubEventsAggregator/internal/model"

	"go.uber.org/zap"
)

type Service interface {
	GetByID(id string) (*model.Event, error)
	GetAll() ([]byte, error)
}

type service struct {
	repo RepoInterface
}

func NewService(r RepoInterface) Service { return &service{repo: r} }

func (s *service) GetByID(id string) (*model.Event, error) { return s.repo.GetEventById(id) }

func (s *service) GetAll() ([]byte, error) {
	if data, hit, err := s.repo.GetAggJson(); hit && err == nil {
		return data, nil
	} else if err != nil {
		return nil, err
	}

	events, err := s.repo.GetAllEvents()
	if err != nil {
		return nil, err
	}
	jsonbytes, err := json.Marshal(events)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetAggJson(jsonbytes, 60); err != nil {
		logger.Lg.Error("warn: cache store failed", zap.Error(err))
	}
	return jsonbytes, nil
}
