package service

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"music-stream-api/pkg/testhelper/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestExternal_ValidateToken_ShouldReturnErrorIfLoginServiceURLIsEmpty(t *testing.T) {
	requestor := &mocks.Requestor{}

	handler := ExternalHandler{
		HttpClient:      requestor,
		LoginServiceURL: "",
	}

	err := handler.ValidateToken("test")
	require.NotNil(t, err)
	require.Equal(t, "login service url cannot be emtpy", err.Error())
}

func TestExternal_ValidateToken_ShouldReturnErrorIfErrorOccursPerformingRequest(t *testing.T) {
	requestor := &mocks.Requestor{}
	requestor.On("Do", mock.Anything).Return(nil, errors.New("test"))

	handler := ExternalHandler{
		HttpClient:      requestor,
		LoginServiceURL: "test",
	}

	err := handler.ValidateToken("test")
	require.NotNil(t, err)
	require.Equal(t, "test", err.Error())
}

func TestExternal_ValidateToken_ShouldReturnErrorIfResponseCodeIsNot200(t *testing.T) {
	requestor := &mocks.Requestor{}
	requestor.On("Do", mock.Anything).Return(&http.Response{StatusCode: http.StatusTeapot}, nil)

	handler := ExternalHandler{
		HttpClient:      requestor,
		LoginServiceURL: "test",
	}

	err := handler.ValidateToken("test")
	require.NotNil(t, err)
	require.Equal(t, fmt.Sprintf("non-200 status code received: %v", http.StatusTeapot), err.Error())
}

func TestExternal_ValidateToken_ShouldReturnNilOnSuccess(t *testing.T) {
	requestor := &mocks.Requestor{}
	requestor.On("Do", mock.Anything).Return(&http.Response{StatusCode: http.StatusOK}, nil)

	handler := ExternalHandler{
		HttpClient:      requestor,
		LoginServiceURL: "test",
	}

	require.Nil(t, handler.ValidateToken("test"))
}
