package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"music-stream-api/pkg/testhelper/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestApi_CheckHealth_ShouldReturn500IfUnableToConnectToDatabase(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("Ping", mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(checkHealth(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_CheckHealth_ShouldReturn200OnSuccess(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("Ping", mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(checkHealth(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
}
