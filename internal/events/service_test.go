package events

import (
	"encoding/json"
	"errors"
	"testing"

	"githubEventsAggregator/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) Save(e *model.Event) error {
	args := m.Called(e)
	return args.Error(0)
}

func (m *mockRepo) GetEventById(id string) (*model.Event, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Event), args.Error(1)
}

func (m *mockRepo) GetAllEvents() ([]model.Event, error) {
	args := m.Called()
	return args.Get(0).([]model.Event), args.Error(1)
}

func (m *mockRepo) GetAggJson() ([]byte, bool, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Bool(1), args.Error(2)
}

func (m *mockRepo) SetAggJson(data []byte, ttl int) error {
	args := m.Called(data, ttl)
	return args.Error(0)
}

func TestService_GetByID(t *testing.T) {
	mockR := new(mockRepo)
	svc := NewService(mockR)

	t.Run("returns event from repo", func(t *testing.T) {
		expected := &model.Event{ID: "123", Type: "PushEvent"}
		mockR.On("GetEventById", "123").Return(expected, nil).Once()

		result, err := svc.GetByID("123")

		assert.NoError(t, err)
		assert.Equal(t, "123", result.ID)
		mockR.AssertExpectations(t)
	})
}

func TestService_GetAll(t *testing.T) {
	mockR := new(mockRepo)
	svc := NewService(mockR)

	t.Run("cache hit", func(t *testing.T) {
		cached := []byte(`[{"id":"1"}]`)
		mockR.On("GetAggJson").Return(cached, true, nil).Once()

		result, err := svc.GetAll()

		assert.NoError(t, err)
		assert.Equal(t, cached, result)
		mockR.AssertNotCalled(t, "GetAllEvents")
		mockR.AssertExpectations(t)
	})

	t.Run("cache miss", func(t *testing.T) {
		events := []model.Event{{ID: "1", Type: "PushEvent"}}
		jsonBytes, _ := json.Marshal(events)

		mockR.On("GetAggJson").Return(nil, false, nil).Once()
		mockR.On("GetAllEvents").Return(events, nil).Once()
		mockR.On("SetAggJson", jsonBytes, 60).Return(nil).Once()

		result, err := svc.GetAll()

		assert.NoError(t, err)
		assert.JSONEq(t, string(jsonBytes), string(result))
		mockR.AssertExpectations(t)
	})

	t.Run("cache error", func(t *testing.T) {
		mockR.On("GetAggJson").Return(nil, false, errors.New("redis down")).Once()

		result, err := svc.GetAll()

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
