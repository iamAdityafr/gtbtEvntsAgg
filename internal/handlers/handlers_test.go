package handlers

import (
	"encoding/json"
	"errors"
	"githubEventsAggregator/internal/model"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockService struct {
	mock.Mock
}

func (ms *mockService) GetByID(id string) (*model.Event, error) {
	args := ms.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Event), args.Error(1)
}
func (ms *mockService) GetAll() ([]byte, error) {
	args := ms.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func TestHTTP_GetEventById(t *testing.T) {
	app := fiber.New()

	mockSvc := new(mockService)
	handler := NewHTTP(mockSvc)

	app.Get("/events/:id", handler.GetEventById)

	t.Run("returns event when found", func(t *testing.T) {
		expectedEvent := &model.Event{ID: "123", Type: "Test Event"}
		mockSvc.On("GetByID", "123").Return(expectedEvent, nil).Once()

		req := httptest.NewRequest("GET", "/events/123", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		var result model.Event
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Equal(t, "123", result.ID)

		mockSvc.AssertExpectations(t)
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		mockSvc.On("GetByID", "999").Return(&model.Event{}, errors.New("not found")).Once()

		req := httptest.NewRequest("GET", "/events/999", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, 404, resp.StatusCode)

		mockSvc.AssertExpectations(t)
	})
}
func TestHTTP_GetAllEvents(t *testing.T) {
	app := fiber.New()
	mockSvc := new(mockService)
	handler := NewHTTP(mockSvc)

	app.Get("/events", handler.GetEvents)
	t.Run("returns all events", func(t *testing.T) {
		exptEvents := []model.Event{
			{ID: "123", Type: "test event1"},
			{ID: "456", Type: "test event2"},
			{ID: "789", Type: "test event3"},
		}

		exptEventsbytes, _ := json.Marshal(exptEvents)
		mockSvc.On("GetAll").Return(exptEventsbytes, nil).Once()
		req := httptest.NewRequest("GET", "/events", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		var res []model.Event
		json.NewDecoder(resp.Body).Decode(&res)

		for i, v := range res {
			assert.Equal(t, exptEvents[i].ID, v.ID)
		}
		mockSvc.AssertExpectations(t)
	})
	t.Run("returns err when not found", func(t *testing.T) {
		mockSvc.On("GetAll").Return([]byte{}, errors.New("not found any event")).Once()
		req := httptest.NewRequest("GET", "/events", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, 404, resp.StatusCode)
		mockSvc.AssertExpectations(t)
	})
}
