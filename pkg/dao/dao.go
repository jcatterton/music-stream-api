package dao

import (
	"context"

	"music-stream-api/pkg/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DbHandler interface {
	Ping(ctx context.Context) error

	AddTrack(ctx context.Context, track models.Track) error
	UploadAudioFile(ctx context.Context, audioFile []byte, trackName string) (interface{}, error)
	DownloadAudioFile(ctx context.Context, audioFileID primitive.ObjectID) ([]byte, error)
	UpdateTrack(ctx context.Context, id primitive.ObjectID, updatedTrack models.Track) error
	GetTracks(ctx context.Context, filters map[string]interface{}) ([]models.Track, error)
	DeleteTrack(ctx context.Context, id primitive.ObjectID) error

	AddPlaylist(ctx context.Context, playlist models.Playlist) error
	UpdatePlaylist(ctx context.Context, playlistId primitive.ObjectID, update bson.M) error
	DeletePlaylist(ctx context.Context, id primitive.ObjectID) error
	GetPlaylists(ctx context.Context, filters map[string]interface{}) ([]models.Playlist, error)
}
