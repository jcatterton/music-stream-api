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

func TestApi_UploadTrack_ShouldReturn400IfUnableToParseForm(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodPost, "/track", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn400IfNoFileWithKeyInputFound(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodPost, "/track", strings.NewReader("{}"))
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn500OnHandlerError(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("UploadAudioFile", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("test"))

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

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn500IfHandlerReturnsInvalidObjectID(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("UploadAudioFile", mock.Anything, mock.Anything, mock.Anything).Return("z", nil)

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

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn500IfErrorOccursAddingTrack(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("UploadAudioFile", mock.Anything, mock.Anything, mock.Anything).Return(primitive.NewObjectID(), nil)
	handler.On("AddTrack", mock.Anything, mock.Anything).Return(errors.New("test"))

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

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_UploadTrack_ShouldReturn200OnSuccessAddingTrack(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("UploadAudioFile", mock.Anything, mock.Anything, mock.Anything).Return(primitive.NewObjectID(), nil)
	handler.On("AddTrack", mock.Anything, mock.Anything).Return(nil)

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

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
}

func TestApi_UploadTrackFromYoutubeLink_ShouldReturn400IfUnableToDecodeRequestBody(t *testing.T) {
	handler := &mocks.DbHandler{}
	client := &mocks.YoutubeClient{}

	req, err := http.NewRequest(http.MethodPost, "/youtube/track", strings.NewReader(""))
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrackFromYoutubeLink(handler, client))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_UploadTrackFromYoutubeLink_ShouldReturnErrorIfGetVideoReturnsError(t *testing.T) {
	handler := &mocks.DbHandler{}
	client := &mocks.YoutubeClient{}
	client.On("GetVideo", mock.Anything).Return(nil, errors.New("test"))

	req, err := http.NewRequest(http.MethodPost, "/youtube/track", strings.NewReader(`{"youtubeLink":"www.youtube.com?v=test&channel=test"}`))
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrackFromYoutubeLink(handler, client))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_UploadTrackFromYoutubeLink_ShouldReturnErrorIfGetStreamReturnsError(t *testing.T) {
	handler := &mocks.DbHandler{}
	client := &mocks.YoutubeClient{}
	client.On("GetVideo", mock.Anything).Return(&youtube.Video{Formats: []youtube.Format{{}}}, nil)
	client.On("GetStream", mock.Anything, mock.Anything).Return(nil, errors.New("test"))

	req, err := http.NewRequest(http.MethodPost, "/youtube/track", strings.NewReader(`{"youtubeLink":"www.youtube.com?v=test&channel=test"}`))
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(uploadTrackFromYoutubeLink(handler, client))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_GetTrackAudio_ShouldReturn400IfUnableToCreateObjectIdFromGivenId(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodGet, "/track/{id}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTrackAudio(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_GetTrackAudio_ShouldReturn500IfGetTracksErrors(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, errors.New("test"))

	req, err := http.NewRequest(http.MethodGet, "/track/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTrackAudio(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_GetTrackAudio_ShouldReturn500IfDownloadAudioFileErrors(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetTracks", mock.Anything, mock.Anything).Return([]models.Track{{AudioFileID: primitive.NewObjectID()}}, nil)
	handler.On("DownloadAudioFile", mock.Anything, mock.Anything).Return(nil, errors.New("test"))

	req, err := http.NewRequest(http.MethodGet, "/track/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTrackAudio(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_GetTrackAudio_ShouldReturn200IfSuccessful(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetTracks", mock.Anything, mock.Anything).Return([]models.Track{{AudioFileID: primitive.NewObjectID()}}, nil)
	handler.On("DownloadAudioFile", mock.Anything, mock.Anything).Return([]byte{}, nil)

	req, err := http.NewRequest(http.MethodGet, "/track/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTrackAudio(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
}

func TestApi_UpdateTrack_ShouldReturn400IfUnableToCreateObjectIdFromGivenId(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodPut, "/track/{id}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(updateTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_UpdateTrack_ShouldReturn500IfUnableToDecodeRequestBody(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodPut, "/track/{id}", ioutil.NopCloser(strings.NewReader("")))
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(updateTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_UpdateTrack_ShouldReturn500IfUpdateTrackErrors(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("UpdateTrack", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodPut, "/track/{id}", ioutil.NopCloser(strings.NewReader("{}")))
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(updateTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_UpdateTrack_ShouldReturn200IfSuccessful(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("UpdateTrack", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPut, "/track/{id}", ioutil.NopCloser(strings.NewReader("{}")))
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(updateTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
}

func TestApi_DeleteTrack_ShouldReturn400IfUnableToCreateObjectIdFromGivenId(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodDelete, "/track/{id}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deleteTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_DeleteTrack_ShouldReturn500IfDeleteTrackErrors(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("DeleteTrack", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodDelete, "/track/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deleteTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_DeleteTrack_ShouldReturn200OnSuccess(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("DeleteTrack", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/track/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deleteTrack(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
}

func TestApi_GetTracks_ShouldReturn500OnGetTracksError(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, errors.New("test"))

	req, err := http.NewRequest(http.MethodGet, "/tracks", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTracks(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_GetTracks_ShouldReturn200OnSuccess(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetTracks", mock.Anything, mock.Anything).Return([]models.Track{{}}, nil)

	req, err := http.NewRequest(http.MethodGet, "/tracks", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getTracks(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
}

func TestApi_AddPlaylist_ShouldReturn400IfUnableToDecodeRequestBody(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodPost, "/playlist", ioutil.NopCloser(strings.NewReader("")))
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_AddPlaylist_ShouldReturn500IfAddPlaylistErrors(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("AddPlaylist", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodPost, "/playlist", ioutil.NopCloser(strings.NewReader("{}")))
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_AddPlaylist_ShouldReturn200OnSuccess(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("AddPlaylist", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/playlist", ioutil.NopCloser(strings.NewReader("{}")))
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn400IfUnableToCreatePlaylistIDFromGivenID(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn400IfUnableToCreateTrackIDFromGivenID(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn500IfGetTracksErrors(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, errors.New("test"))

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn500IfUpdatePlaylistErrors(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, nil)
	handler.On("UpdatePlaylist", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_AddTrackToPlaylist_ShouldReturn200OnSuccess(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, nil)
	handler.On("UpdatePlaylist", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodPost, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(addTrackToPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn400IfUnableToCreatePlaylistIDFromGivenID(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn400IfUnableToCreateTrackIDFromGivenID(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn500IfGetTracksErrors(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, errors.New("test"))

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn500IfUpdatePlaylistErrors(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, nil)
	handler.On("UpdatePlaylist", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_RemoveTrackFromPlaylist_ShouldReturn200OnSuccess(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetTracks", mock.Anything, mock.Anything).Return(nil, nil)
	handler.On("UpdatePlaylist", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{playlistId}/track/{trackId}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"playlistid": "603ac4abd9ad8067f54a2778", "trackid": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(removeTrackFromPlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
}

func TestApi_DeletePlaylist_ShouldReturn400IfUnableToGetObjectIDFromGivenID(t *testing.T) {
	handler := &mocks.DbHandler{}

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{id}", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deletePlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 400, recorder.Code)
}

func TestApi_DeletePlaylist_ShouldReturn500IfDeletePlaylistErrors(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("DeletePlaylist", mock.Anything, mock.Anything).Return(errors.New("test"))

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deletePlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_DeletePlaylist_ShouldReturn200IfSuccessful(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("DeletePlaylist", mock.Anything, mock.Anything).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/playlist/{id}", nil)
	require.Nil(t, err)
	req = mux.SetURLVars(req, map[string]string{"id": "603ac4abd9ad8067f54a2778"})

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(deletePlaylist(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
}

func TestApi_GetPlaylists_ShouldReturn500IfGetPlaylistErrors(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetPlaylists", mock.Anything, mock.Anything).Return(nil, errors.New("test"))

	req, err := http.NewRequest(http.MethodGet, "/playlists", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getPlaylists(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 500, recorder.Code)
}

func TestApi_GetPlaylists_ShouldReturn200OnSuccess(t *testing.T) {
	handler := &mocks.DbHandler{}
	handler.On("GetPlaylists", mock.Anything, mock.Anything).Return([]models.Playlist{{}}, nil)

	req, err := http.NewRequest(http.MethodGet, "/playlists", nil)
	require.Nil(t, err)

	recorder := httptest.NewRecorder()
	httpHandler := http.HandlerFunc(getPlaylists(handler))
	httpHandler.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
}
