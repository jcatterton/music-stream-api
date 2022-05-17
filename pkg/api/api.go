package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"music-stream-api/pkg/service"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"music-stream-api/pkg/dao"
	"music-stream-api/pkg/models"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/kkdai/youtube/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type YoutubeClient interface {
	GetVideo(videoId string) (*youtube.Video, error)
	GetStream(video *youtube.Video, format *youtube.Format) (io.ReadCloser, int64, error)
}

func ListenAndServe() error {
	headers := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type"})
	origins := handlers.AllowedOrigins([]string{"*"})
	methods := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"})

	router, err := route()
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler:      handlers.CORS(headers, origins, methods)(router),
		Addr:         ":8002",
		WriteTimeout: 20 * time.Second,
		ReadTimeout:  20 * time.Second,
	}
	shutdownGracefully(server)

	logrus.Info("Starting API server...")
	return server.ListenAndServe()
}

func route() (*mux.Router, error) {
	dbClient, err := mongo.Connect(context.Background(), options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		logrus.WithError(err).Error("Error creating database client")
		return nil, err
	}

	dbHandler := dao.DatabaseHandler{
		Client:               dbClient,
		Database:             "db",
		TrackCollection:      "songs",
		PlaylistCollection:   "playlists",
		AudioCollection:      "fs.files",
		AudioChunkCollection: "fs.chunks",
	}

	client := youtube.Client{}

	extHandler := service.ExternalHandler{
		LoginServiceURL: os.Getenv("LOGIN_URL"),
		HttpClient:      http.DefaultClient,
	}

	r := mux.NewRouter()

	r.HandleFunc("/health", checkHealth(&dbHandler)).Methods(http.MethodGet)

	r.HandleFunc("/track", uploadTrack(&dbHandler, &extHandler)).Methods(http.MethodPost)
	r.HandleFunc("/track/{id}", getTrackAudio(&dbHandler, &extHandler)).Methods(http.MethodGet)
	r.HandleFunc("/track/{id}", updateTrack(&dbHandler, &extHandler)).Methods(http.MethodPut)
	r.HandleFunc("/track/{id}", deleteTrack(&dbHandler, &extHandler)).Methods(http.MethodDelete)
	r.HandleFunc("/tracks", getTracks(&dbHandler, &extHandler)).Methods(http.MethodGet)
	r.HandleFunc("/video", getVideo(&extHandler, &client)).Methods(http.MethodPost)
	r.HandleFunc("/stream", getStream(&extHandler, &client)).Methods(http.MethodPost)
	r.HandleFunc("/convert", convertStreamToAudio(&extHandler)).Methods(http.MethodPost)
	r.HandleFunc("/upload", uploadAudioBytes(&dbHandler, &extHandler)).Methods(http.MethodPost)

	r.HandleFunc("/playlist", addPlaylist(&dbHandler, &extHandler)).Methods(http.MethodPost)
	r.HandleFunc("/playlist/{playlistid}/track/{trackid}", addTrackToPlaylist(&dbHandler, &extHandler)).Methods(http.MethodPost)
	r.HandleFunc("/playlist/{playlistid}/track/{trackid}", removeTrackFromPlaylist(&dbHandler, &extHandler)).Methods(http.MethodDelete)
	r.HandleFunc("/playlist/{id}", deletePlaylist(&dbHandler, &extHandler)).Methods(http.MethodDelete)
	r.HandleFunc("/playlists", getPlaylists(&dbHandler, &extHandler)).Methods(http.MethodGet)

	//Deprecated
	r.HandleFunc("/youtube/track", uploadTrackFromYoutubeLink(&dbHandler, &client, &extHandler)).Methods(http.MethodPost)

	return r, nil
}

func checkHealth(handler dao.DbHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer closeRequestBody(r)
		if err := handler.Ping(r.Context()); err != nil {
			respondWithError(w, http.StatusInternalServerError, "API is running but unable to connect to database")
			return
		}
		respondWithSuccess(w, http.StatusOK, "API is running and connected to database")
		return
	}
}

func uploadTrack(handler dao.DbHandler, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		if err := r.ParseForm(); err != nil {
			logrus.WithError(err).Error("Error parsing request form")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		f, _, err := r.FormFile("input")
		if err != nil {
			logrus.WithError(err).Error("Failed to find file with key 'input'")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		buf := bytes.NewBuffer(nil)
		if _, err := io.Copy(buf, f); err != nil {
			logrus.WithError(err).Error("Error reading file")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		defer func() {
			closeRequestBody(r)
			if err = f.Close(); err != nil {
				logrus.WithError(err).Error("Error closing file")
			}
		}()

		body := r.FormValue("body")
		track := models.Track{}
		if err := json.Unmarshal([]byte(body), &track); err != nil {
			logrus.WithError(err).Error("Error reading request body")
			respondWithError(w, http.StatusBadRequest, err.Error())
		}

		track.ID = primitive.NewObjectID()
		if track.Name == "" {
			track.Name = "Unknown"
		}
		if track.Artist == "" {
			track.Artist = "Unknown Artist"
		}
		if track.AlbumName == "" {
			track.AlbumName = "Unknown Album"
		}

		audioID, err := handler.UploadAudioFile(ctx, buf.Bytes(), track.Name)
		if err != nil {
			logrus.WithError(err).Error("Error adding track to database")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if _, ok := audioID.(primitive.ObjectID); !ok {
			logrus.WithError(err).Error("Did not receive valid audioFileID from upload stream")
			respondWithError(w, http.StatusInternalServerError, "invalid audioID received from handler")
			return
		}
		track.AudioFileID = audioID.(primitive.ObjectID)

		if err := handler.AddTrack(ctx, track); err != nil {
			logrus.WithError(err).Error("Error adding track to database")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, "Track added successfully")
		return
	}
}

func getVideo(ext service.ExtHandler, client YoutubeClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		var ytRequest models.YoutubeRequest
		if err := json.NewDecoder(r.Body).Decode(&ytRequest); err != nil {
			logrus.WithError(err).Error("Error decoding request into JSON")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		videoId := strings.Split(strings.Split(ytRequest.YoutubeLink, "v=")[1], "&")[0]

		video, err := client.GetVideo(videoId)
		if err != nil {
			logrus.WithError(err).Error("Error getting video")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, video)
	}
}

func getStream(ext service.ExtHandler, client YoutubeClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		var video youtube.Video
		if err := json.NewDecoder(r.Body).Decode(&video); err != nil {
			logrus.WithError(err).Error("Error decoding request body")
			respondWithError(w, http.StatusBadRequest, "Error decoding request body")
			return
		}

		formatIndex := 0
		for i, format := range video.Formats {
			if strings.Contains(format.MimeType, "audio/mp4") {
				formatIndex = i
				break
			}
		}

		stream, _, err := client.GetStream(&video, &video.Formats[formatIndex])
		if err != nil {
			logrus.WithError(err).Error("Error getting video stream")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		file, err := os.Create("video.mp4")
		if err != nil {
			logrus.WithError(err).Error("Error creating file")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		defer func() {
			if err := file.Close(); err != nil {
				logrus.WithError(err).Error("Error closing file")
			}
			if err := stream.Close(); err != nil {
				logrus.WithError(err).Error("Error closing stream")
			}
		}()

		if _, err = io.Copy(file, stream); err != nil {
			logrus.WithError(err).Error("Error encoding response body")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, "Stream file created successfully")
	}
}

func convertStreamToAudio(ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		ffmpeg, err := exec.LookPath("ffmpeg")
		if err != nil {
			logrus.WithError(err).Error("Error locating ffmpeg")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		cmd := exec.Command(ffmpeg, "-y", "-loglevel", "quiet", "-i", "video.mp4", "video.mp3")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			logrus.WithError(err).Error("Error executing ffmpeg command")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		audioBytes, err := ioutil.ReadFile("video.mp3")
		if err != nil {
			logrus.WithError(err).Error("Error reading file")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if err = os.Remove("video.mp4"); err != nil {
			logrus.WithError(err).Error("Error deleting video file")
		}
		if err = os.Remove("video.mp3"); err != nil {
			logrus.WithError(err).Error("Error deleting audio file")
		}

		respondWithSuccessBytes(w, http.StatusOK, audioBytes)
	}
}

func uploadAudioBytes(handler dao.DbHandler, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		var uploadRequest models.UploadRequest
		if err := json.NewDecoder(r.Body).Decode(&uploadRequest); err != nil {
			logrus.WithError(err).Error("Error decoding request body")
			respondWithError(w, http.StatusBadRequest, "Error decoding request body")
			return
		}

		track := models.Track{
			ID:        primitive.NewObjectID(),
			Name:      uploadRequest.YoutubeRequest.Name,
			Artist:    uploadRequest.YoutubeRequest.Artist,
			AlbumName: uploadRequest.YoutubeRequest.AlbumName,
		}

		if track.Name == "" {
			track.Name = "Unknown"
		}
		if track.Artist == "" {
			track.Artist = "Unknown Artist"
		}
		if track.AlbumName == "" {
			track.AlbumName = "Unknown Album"
		}

		audioID, err := handler.UploadAudioFile(ctx, uploadRequest.AudioBytes, track.Name)
		if err != nil {
			logrus.WithError(err).Error("Error adding track to database")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if _, ok := audioID.(primitive.ObjectID); !ok {
			logrus.WithError(err).Error("Did not receive valid audioFileID from upload stream")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		track.AudioFileID = audioID.(primitive.ObjectID)

		if err := handler.AddTrack(ctx, track); err != nil {
			logrus.WithError(err).Error("Error adding track to database")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, "Track added successfully")
		return
	}
}

func getTrackAudio(handler dao.DbHandler, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := mux.Vars(r)["id"]

		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		objectID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			logrus.WithError(err).Error("Error creating objectID")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		filter := map[string]interface{}{"_id": objectID}
		tracks, err := handler.GetTracks(ctx, filter)
		if err != nil {
			logrus.WithError(err).Error("Error getting track")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		audioFileBytes, err := handler.DownloadAudioFile(ctx, tracks[0].AudioFileID)
		if err != nil {
			logrus.WithError(err).Error("Error getting audio for track")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		reader := bytes.NewReader(audioFileBytes)
		if _, err := io.Copy(w, reader); err != nil {
			logrus.WithError(err).Error("Error writing file to response")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

func updateTrack(handler dao.DbHandler, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
		if err != nil {
			logrus.WithError(err).Error("Error creating objectID from hex")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		var updatedTrack models.Track
		if err := json.NewDecoder(r.Body).Decode(&updatedTrack); err != nil {
			logrus.WithError(err).Error("Error decoding request body")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if updatedTrack.Name == "" {
			updatedTrack.Name = "Unknown"
		}
		if updatedTrack.Artist == "" {
			updatedTrack.Artist = "Unknown Artist"
		}
		if updatedTrack.AlbumName == "" {
			updatedTrack.AlbumName = "Unknown Album"
		}

		if err := handler.UpdateTrack(ctx, id, updatedTrack); err != nil {
			logrus.WithError(err).Error("Error updating track in database")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, "Track updated successfully")
		return
	}
}

func deleteTrack(handler dao.DbHandler, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
		if err != nil {
			logrus.WithError(err).Error("Error creating objectID from hex")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := handler.DeleteTrack(ctx, id); err != nil {
			logrus.WithError(err).Error("Error deleting track")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, "Track deleted successfully")
		return
	}
}

func getTracks(handler dao.DbHandler, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		if err := r.ParseForm(); err != nil {
			logrus.WithError(err).Error("Error parsing request form")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		filters := make(map[string]interface{})
		query := r.URL.Query()
		for key, val := range query {
			filters[key] = val[0]
		}

		trackList, err := handler.GetTracks(ctx, filters)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving tracks")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, trackList)
		return
	}
}

func addPlaylist(handler dao.DbHandler, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		var playlist models.Playlist
		if err := json.NewDecoder(r.Body).Decode(&playlist); err != nil {
			logrus.WithError(err).Error("Error decoding request body")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		playlist.ID = primitive.NewObjectID()

		if err := handler.AddPlaylist(ctx, playlist); err != nil {
			logrus.WithError(err).Error("Error creating playlist")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, "Playlist created successfully")
		return
	}
}

func addTrackToPlaylist(handler dao.DbHandler, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		playlistId := mux.Vars(r)["playlistid"]
		trackId := mux.Vars(r)["trackid"]

		pid, err := primitive.ObjectIDFromHex(playlistId)
		if err != nil {
			logrus.WithError(err).Error("Error creating objectId from hex")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		tid, err := primitive.ObjectIDFromHex(trackId)
		if err != nil {
			logrus.WithError(err).Error("Error creating objectId from hex")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		_, err = handler.GetTracks(ctx, map[string]interface{}{"_id": tid})
		if err != nil {
			logrus.WithError(err).Error("No track with given ID found in database")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		update := bson.M{"$push": bson.M{"tracks": tid}}
		if err := handler.UpdatePlaylist(ctx, pid, update); err != nil {
			logrus.WithError(err).Error("Error adding track to playlist")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, "Track successfully added to playlist")
		return
	}
}

func removeTrackFromPlaylist(handler dao.DbHandler, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		playlistId := mux.Vars(r)["playlistid"]
		trackId := mux.Vars(r)["trackid"]

		pid, err := primitive.ObjectIDFromHex(playlistId)
		if err != nil {
			logrus.WithError(err).Error("Error creating objectId from hex")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		tid, err := primitive.ObjectIDFromHex(trackId)
		if err != nil {
			logrus.WithError(err).Error("Error creating objectId from hex")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		_, err = handler.GetTracks(ctx, map[string]interface{}{"_id": tid})
		if err != nil {
			logrus.WithError(err).Error("No track with given ID found in database")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		update := bson.M{"$pull": bson.M{"tracks": tid}}
		if err := handler.UpdatePlaylist(ctx, pid, update); err != nil {
			logrus.WithError(err).Error("Error removing track from playlist")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, "Track successfully removed from playlist")
		return
	}
}

func deletePlaylist(handler dao.DbHandler, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
		if err != nil {
			logrus.WithError(err).Error("Error creating objectID from hex")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := handler.DeletePlaylist(ctx, id); err != nil {
			logrus.WithError(err).Error("Error deleting track")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, "Playlist deleted successfully")
		return
	}
}

func getPlaylists(handler dao.DbHandler, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		if err := r.ParseForm(); err != nil {
			logrus.WithError(err).Error("Error parsing request form")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		filters := make(map[string]interface{})
		query := r.URL.Query()
		for key, val := range query {
			filters[key] = val[0]
		}

		playlists, err := handler.GetPlaylists(ctx, filters)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving tracks")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, playlists)
		return
	}
}

func shutdownGracefully(server *http.Server) {
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt)
		<-signals

		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(c); err != nil {
			logrus.WithError(err).Error("Error shutting down server")
		}

		<-c.Done()
		os.Exit(0)
	}()
}

func respondWithSuccess(w http.ResponseWriter, code int, body interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if body == nil {
		logrus.Error("Body is nil, unable to write response")
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logrus.WithError(err).Error("Error encoding response")
	}
}

func respondWithSuccessBytes(w http.ResponseWriter, code int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if body == nil {
		logrus.Error("Body is nil, unable to write response")
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logrus.WithError(err).Error("Error encoding response")
	}
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if message == "" {
		logrus.Error("Body is nil, unable to write response")
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		logrus.WithError(err).Error("Error encoding response")
	}
}

func closeRequestBody(req *http.Request) {
	if req.Body == nil {
		return
	}
	if err := req.Body.Close(); err != nil {
		logrus.WithError(err).Error("Error closing request body")
		return
	}
	return
}

func getAuthToken(r *http.Request) (string, error) {
	tokenHeader := r.Header.Get("Authorization")
	if tokenHeader == "" {
		return "", errors.New("no authorization header found")
	} else if (len(tokenHeader) >= 7 && tokenHeader[:7] != "Bearer ") || len(strings.Split(tokenHeader, " ")) != 2 {
		return "", errors.New("authorization header must be in format 'Bearer' <token>")
	}
	return strings.Split(tokenHeader, " ")[1], nil
}

// Deprecated
func uploadTrackFromYoutubeLink(handler dao.DbHandler, client YoutubeClient, ext service.ExtHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer closeRequestBody(r)

		token, err := getAuthToken(r)
		if err != nil {
			logrus.WithError(err).Error("Error retrieving auth token")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ext.ValidateToken(token); err != nil {
			logrus.WithError(err).Error("Authentication failed")
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		var ytRequest models.YoutubeRequest
		if err := json.NewDecoder(r.Body).Decode(&ytRequest); err != nil {
			logrus.WithError(err).Error("Error decoding request into JSON")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		videoId := strings.Split(strings.Split(ytRequest.YoutubeLink, "v=")[1], "&")[0]

		video, err := client.GetVideo(videoId)
		if err != nil {
			logrus.WithError(err).Error("Error getting video")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		formatIndex := 0
		for i, format := range video.Formats {
			if strings.Contains(format.MimeType, "audio/mp4") {
				formatIndex = i
				break
			}
		}

		stream, _, err := client.GetStream(video, &video.Formats[formatIndex])
		if err != nil {
			logrus.WithError(err).Error("Error getting video stream")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		file, err := os.Create("video.mp4")
		if err != nil {
			logrus.WithError(err).Error("Error creating file")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		defer func() {
			if err := file.Close(); err != nil {
				logrus.WithError(err).Error("Error closing file")
			}
			if err := stream.Close(); err != nil {
				logrus.WithError(err).Error("Error closing stream")
			}
		}()

		if _, err = io.Copy(file, stream); err != nil {
			logrus.WithError(err).Error("Error encoding response body")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		ffmpeg, err := exec.LookPath("ffmpeg")
		if err != nil {
			logrus.WithError(err).Error("Error locating ffmpeg")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		cmd := exec.Command(ffmpeg, "-y", "-loglevel", "quiet", "-i", "video.mp4", "video.mp3")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			logrus.WithError(err).Error("Error executing ffmpeg command")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		audioBytes, err := ioutil.ReadFile("video.mp3")
		if err != nil {
			logrus.WithError(err).Error("Error reading file")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if err = os.Remove("video.mp4"); err != nil {
			logrus.WithError(err).Error("Error deleting video file")
		}
		if err = os.Remove("video.mp3"); err != nil {
			logrus.WithError(err).Error("Error deleting audio file")
		}

		track := models.Track{
			ID:        primitive.NewObjectID(),
			Name:      ytRequest.Name,
			Artist:    ytRequest.Artist,
			AlbumName: ytRequest.AlbumName,
		}

		if track.Name == "" {
			track.Name = "Unknown"
		}
		if track.Artist == "" {
			track.Artist = "Unknown Artist"
		}
		if track.AlbumName == "" {
			track.AlbumName = "Unknown Album"
		}

		audioID, err := handler.UploadAudioFile(ctx, audioBytes, track.Name)
		if err != nil {
			logrus.WithError(err).Error("Error adding track to database")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if _, ok := audioID.(primitive.ObjectID); !ok {
			logrus.WithError(err).Error("Did not receive valid audioFileID from upload stream")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		track.AudioFileID = audioID.(primitive.ObjectID)

		if err := handler.AddTrack(ctx, track); err != nil {
			logrus.WithError(err).Error("Error adding track to database")
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithSuccess(w, http.StatusOK, "Track added successfully")
		return
	}
}
