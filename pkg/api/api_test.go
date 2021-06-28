package api

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"music-stream-api/pkg/models"
	"music-stream-api/pkg/testhelper/mocks"

	"github.com/gorilla/mux"
	"github.com/kkdai/youtube/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestApi_CheckHealth_ShouldReturn500IfUnableToConnectToDatabase(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	dbHandler.On("Ping", mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(checkHealth(dbHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_CheckHealth_ShouldReturn200OnSuccess(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	dbHandler.On("Ping", mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(checkHealth(dbHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn400IfNoAuthorizationHeaderFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}

	req, err := http.NewRequest(http.MethodPost, "/track", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn401IfErrorOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodPost, "/track", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn400IfErrorOccursParsingForm(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/track", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn400IfNoFileWithKeyInputFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/track", strings.NewReader("{}"))
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn500OnHandlerError(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("UploadAudioFile", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("input", "test.mp3")
	require.Nil(t, err)

	_, err = io.Copy(part, bytes.NewBuffer([]byte("test")))
	require.Nil(t, err)

	require.Nil(t, writer.WriteField("body", "{}"))

	require.Nil(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/track", body)
	require.Nil(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn500IfHandlerReturnsInvalidObjectID(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("UploadAudioFile", mock.Anything, mock.Anything, mock.Anything).Return("z", nil)
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("input", "test.mp3")
	require.Nil(t, err)

	_, err = io.Copy(part, bytes.NewBuffer([]byte("test")))
	require.Nil(t, err)

	require.Nil(t, writer.WriteField("body", "{}"))

	require.Nil(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/track", body)
	require.Nil(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn500IfErrorOccursAddingTrack(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("UploadAudioFile", mock.Anything, mock.Anything, mock.Anything).Return(primitive.NewObjectID(), nil)
	dbHandler.On("AddTrack", mock.Anything, mock.Anything).Return(errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("input", "test.mp3")
	require.Nil(t, err)

	_, err = io.Copy(part, bytes.NewBuffer([]byte("test")))
	require.Nil(t, err)

	require.Nil(t, writer.WriteField("body", "{}"))

	require.Nil(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/track", body)
	require.Nil(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn200OnSuccessAddingTrack(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("UploadAudioFile", mock.Anything, mock.Anything, mock.Anything).Return(primitive.NewObjectID(), nil)
	dbHandler.On("AddTrack", mock.Anything, mock.Anything).Return(nil)
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("input", "test.mp3")
	require.Nil(t, err)

	_, err = io.Copy(part, bytes.NewBuffer([]byte("test")))
	require.Nil(t, err)

	require.Nil(t, writer.WriteField("body", "{}"))

	require.Nil(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/track", body)
	require.Nil(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestApi_UploadTrackFromYoutubeLink_ShouldReturn400IfNoAuthorizationHeaderFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	client := &mocks.YoutubeClient{}

	req, err := http.NewRequest(http.MethodPost, "/youtube/track", strings.NewReader(""))
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrackFromYoutubeLink(dbHandler, client, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_UploadTrackFromYoutubeLink_ShouldReturn401IfErrorOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	client := &mocks.YoutubeClient{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodPost, "/youtube/track", strings.NewReader(""))
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrackFromYoutubeLink(dbHandler, client, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestApi_UploadTrackFromYoutubeLink_ShouldReturn400IfErrorOccursDecodingRequestBody(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	client := &mocks.YoutubeClient{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/youtube/track", strings.NewReader(""))
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrackFromYoutubeLink(dbHandler, client, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_UploadTrackFromYoutubeLink_ShouldReturnErrorIfGetVideoReturnsError(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	client := &mocks.YoutubeClient{}
	client.On("GetVideo", mock.Anything).Return(nil, errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/youtube/track", strings.NewReader(`{"youtubeLink":"www.youtube.com?v=test&channel=test"}`))
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrackFromYoutubeLink(dbHandler, client, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_UploadTrackFromYoutubeLink_ShouldReturnErrorIfGetStreamReturnsError(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	client := &mocks.YoutubeClient{}
	client.On("GetVideo", mock.Anything).Return(&youtube.Video{Formats: []youtube.Format{{}}}, nil)
	client.On("GetStream", mock.Anything, mock.Anything).Return(nil, int64(0), errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/youtube/track", strings.NewReader(`{"youtubeLink":"www.youtube.com?v=test&channel=test"}`))
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrackFromYoutubeLink(dbHandler, client, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_GetTrackAudio_ShouldReturn400IfNoAuthorizationHeaderFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}

	req, err := http.NewRequest(http.MethodGet, "/track/{id}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTrackAudio(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_GetTrackAudio_ShouldReturn401IfErrorOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodGet, "/track/{id}", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTrackAudio(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestApi_GetTrackAudio_ShouldReturn400IfUnableToCreateObjectIDFromGivenID(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodGet, "/track/{id}", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTrackAudio(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_GetTrackAudio_ShouldReturn500IfGetTracksErrors(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodGet, "/track/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTrackAudio(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_GetTrackAudio_ShouldReturn500IfDownloadAudioFileErrors(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetTracks", mock.Anything, mock.Anything).Return([]models.Track{{AudioFileID: primitive.NewObjectID()}}, nil)
	dbHandler.On("DownloadAudioFile", mock.Anything, mock.Anything).Return(nil, errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodGet, "/track/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTrackAudio(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_GetTrackAudio_ShouldReturn200IfSuccessful(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetTracks", mock.Anything, mock.Anything).Return([]models.Track{{AudioFileID: primitive.NewObjectID()}}, nil)
	dbHandler.On("DownloadAudioFile", mock.Anything, mock.Anything).Return([]byte{}, nil)
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodGet, "/track/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTrackAudio(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestApi_UpdateTrack_ShouldReturn400IfNoAuthorizationHeaderFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}

	req, err := http.NewRequest(http.MethodPut, "/track/{id}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(updateTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_UpdateTrack_ShouldReturn401IfErrorOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodPut, "/track/{id}", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(updateTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestApi_UpdateTrack_ShouldReturn400IfUnableToCreateObjectIDFromGivenID(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPut, "/track/{id}", ioutil.NopCloser(strings.NewReader("")))
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(updateTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_UpdateTrack_ShouldReturn500IfUnableToDecodeRequestBody(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPut, "/track/{id}", ioutil.NopCloser(strings.NewReader("")))
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(updateTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_UpdateTrack_ShouldReturn500IfUpdateTrackErrors(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("UpdateTrack", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPut, "/track/{id}", ioutil.NopCloser(strings.NewReader("{}")))
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(updateTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_UpdateTrack_ShouldReturn200IfSuccessful(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("UpdateTrack", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPut, "/track/{id}", ioutil.NopCloser(strings.NewReader("{}")))
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(updateTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestApi_DeleteTrack_ShouldReturn400IfNoAuthorizationHeaderFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}

	req, err := http.NewRequest(http.MethodDelete, "/track/{id}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deleteTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_DeleteTrack_ShouldReturn401IfErrorsOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodDelete, "/track/{id}", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deleteTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestApi_DeleteTrack_ShouldReturn400IfUnableToCreateObjectIDFromGivenID(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/track/{id}", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deleteTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_DeleteTrack_ShouldReturn500IfDeleteTrackErrors(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("DeleteTrack", mock.Anything, mock.Anything).Return(errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/track/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deleteTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_DeleteTrack_ShouldReturn200OnSuccess(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("DeleteTrack", mock.Anything, mock.Anything).Return(nil)
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/track/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deleteTrack(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestApi_GetTracks_ShouldReturn400IfNoAuthorizationHeaderFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}

	req, err := http.NewRequest(http.MethodGet, "/tracks", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTracks(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_GetTracks_ShouldReturn401IfErrorOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodGet, "/tracks", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTracks(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestApi_GetTracks_ShouldReturn500OnGetTracksError(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodGet, "/tracks", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTracks(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_GetTracks_ShouldReturn200OnSuccess(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetTracks", mock.Anything, mock.Anything).Return([]models.Track{{}}, nil)
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodGet, "/tracks", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTracks(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestApi_AddPlaylist_ShouldReturn400IfNoAuthorizationHeaderFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}

	req, err := http.NewRequest(http.MethodPost, "/playlist", ioutil.NopCloser(strings.NewReader("")))
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_AddPlaylist_ShouldReturn401IfErrorOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodPost, "/playlist", ioutil.NopCloser(strings.NewReader("")))
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestApi_AddPlaylist_ShouldReturn400IfErrorOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/playlist", ioutil.NopCloser(strings.NewReader("")))
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_AddPlaylist_ShouldReturn500IfAddPlaylistErrors(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("AddPlaylist", mock.Anything, mock.Anything).Return(errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/playlist", ioutil.NopCloser(strings.NewReader("{}")))
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_AddPlaylist_ShouldReturn200OnSuccess(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("AddPlaylist", mock.Anything, mock.Anything).Return(nil)
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/playlist", ioutil.NopCloser(strings.NewReader("{}")))
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn400IfNoAuthorizationHeaderFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn401IfErrorOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn400IfUnableToCreatePlaylistIDFromGivenID(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn400IfUnableToCreateTrackIDFromGivenID(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn500IfGetTracksErrors(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn500IfUpdatePlaylistErrors(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, nil)
	dbHandler.On("UpdatePlaylist", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn200OnSuccess(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, nil)
	dbHandler.On("UpdatePlaylist", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn400IfNoAuthorizationHeaderFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn401IfErrorOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn400IfUnableToCreatePlaylistIDFromGivenID(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn400IfUnableToCreateTrackIDFromGivenID(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn500IfGetTracksErrors(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn500IfUpdatePlaylistErrors(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, nil)
	dbHandler.On("UpdatePlaylist", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn200OnSuccess(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, nil)
	dbHandler.On("UpdatePlaylist", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestApi_DeletePlaylist_ShouldReturn400IfNoAuthorizationHeaderFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{id}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deletePlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_DeletePlaylist_ShouldReturn401IfErrorOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{id}", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deletePlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestApi_DeletePlaylist_ShouldReturn400IfUnableToCreateObjectIDFromGivenID(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{id}", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deletePlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_DeletePlaylist_ShouldReturn500IfDeletePlaylistErrors(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("DeletePlaylist", mock.Anything, mock.Anything).Return(errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deletePlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_DeletePlaylist_ShouldReturn200IfSuccessful(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("DeletePlaylist", mock.Anything, mock.Anything).Return(nil)
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deletePlaylist(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestApi_GetPlaylists_ShouldReturn400IfNoAuthorizationHeaderFound(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}

	req, err := http.NewRequest(http.MethodGet, "/playlists", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getPlaylists(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApi_GetPlaylists_ShouldReturn401IfErrorOccursValidatingToken(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodGet, "/playlists", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getPlaylists(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestApi_GetPlaylists_ShouldReturn500IfGetPlaylistErrors(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetPlaylists", mock.Anything, mock.Anything).Return(nil, errors.New("test"))
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodGet, "/playlists", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getPlaylists(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApi_GetPlaylists_ShouldReturn200OnSuccess(t *testing.T) {
	dbHandler := &mocks.DbHandler{}
	extHandler := &mocks.ExtHandler{}
	dbHandler.On("GetPlaylists", mock.Anything, mock.Anything).Return([]models.Playlist{{}}, nil)
	extHandler.On("ValidateToken", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodGet, "/playlists", nil)
	require.Nil(t, err)
	req.Header.Set("Authorization", "Bearer test")

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getPlaylists(dbHandler, extHandler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}
