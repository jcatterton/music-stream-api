package dao

import (
	"bytes"
	"context"
	"errors"

	"music-stream-api/pkg/models"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoClient struct {
	Client               *mongo.Client
	Database             string
	TrackCollection      string
	PlaylistCollection   string
	AudioCollection      string
	AudioChunkCollection string
}

func (db *MongoClient) getTrackCollection() *mongo.Collection {
	return db.Client.Database(db.Database).Collection(db.TrackCollection)
}

func (db *MongoClient) getPlaylistCollection() *mongo.Collection {
	return db.Client.Database(db.Database).Collection(db.PlaylistCollection)
}

func (db *MongoClient) getAudioCollection() *mongo.Collection {
	return db.Client.Database(db.Database).Collection(db.AudioCollection)
}

func (db *MongoClient) getAudioChunkCollection() *mongo.Collection {
	return db.Client.Database(db.Database).Collection(db.AudioChunkCollection)
}

func (db *MongoClient) GetTracks(ctx context.Context, filters map[string]interface{}) ([]models.Track, error) {
	cursor, err := db.getTrackCollection().Find(ctx, filters)
	if err != nil {
		return nil, err
	}

	var results []models.Track
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (db *MongoClient) UploadAudioFile(ctx context.Context, audioFile []byte, trackName string) (interface{}, error) {
	bucket, err := gridfs.NewBucket(db.Client.Database(db.Database))
	if err != nil {
		return nil, err
	}

	uploadStream, err := bucket.OpenUploadStream(trackName)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := uploadStream.Close(); err != nil {
			logrus.WithError(err).Error("Error closing upload stream")
		}
	}()

	_, err = uploadStream.Write(audioFile)
	if err != nil {
		return nil, err
	}

	return uploadStream.FileID, nil
}

func (db *MongoClient) AddTrack(ctx context.Context, track models.Track) error {
	results, err := db.getTrackCollection().InsertOne(ctx, track)
	if err != nil {
		return err
	} else if results.InsertedID == nil {
		return errors.New("no tracks inserted")
	}
	return nil
}

func (db *MongoClient) DownloadAudioFile(ctx context.Context, audioFileID primitive.ObjectID) ([]byte, error) {
	bucket, err := gridfs.NewBucket(db.Client.Database(db.Database))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	_, err = bucket.DownloadToStream(audioFileID, &buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (db *MongoClient) UpdateTrack(ctx context.Context, id primitive.ObjectID, updatedTrack models.Track) error {
	filter := map[string]interface{}{"_id": id}

	findResult := db.getTrackCollection().FindOne(ctx, filter)
	if findResult.Err() != nil {
		return findResult.Err()
	}

	var track models.Track
	if err := findResult.Decode(&track); err != nil {
		return err
	}

	if updatedTrack.Name != "" {
		track.Name = updatedTrack.Name
	}
	if updatedTrack.Artist != "" {
		track.Artist = updatedTrack.Artist
	}
	if updatedTrack.AlbumName != "" {
		track.AlbumName = updatedTrack.AlbumName
	}

	updateResult := db.getTrackCollection().FindOneAndUpdate(ctx, filter, bson.M{"$set": track})
	if updateResult.Err() != nil {
		return updateResult.Err()
	}

	return nil
}

func (db *MongoClient) DeleteTrack(ctx context.Context, id primitive.ObjectID) error {
	filter := map[string]interface{}{"_id": id}

	result := db.getTrackCollection().FindOneAndDelete(ctx, filter)
	if result.Err() != nil {
		return result.Err()
	}

	var track models.Track
	if err := result.Decode(&track); err != nil {
		return err
	}

	_, err := db.getAudioCollection().DeleteOne(ctx, map[string]interface{}{"_id": track.AudioFileID})
	if err != nil {
		return err
	}

	_, err = db.getAudioChunkCollection().DeleteMany(ctx, map[string]interface{}{"files_id": track.AudioFileID})
	if err != nil {
		return err
	}

	_, err = db.getPlaylistCollection().UpdateMany(ctx,
		bson.M{"tracks": track.ID},
		bson.M{"$pull": bson.M{"tracks": track.ID}},
	)

	return nil
}

func (db *MongoClient) AddPlaylist(ctx context.Context, playlist models.Playlist) error {
	results, err := db.getPlaylistCollection().InsertOne(ctx, playlist)
	if err != nil {
		return err
	} else if results.InsertedID == nil {
		return errors.New("no playlist inserted")
	}
	return nil
}

func (db *MongoClient) UpdatePlaylist(ctx context.Context, playlistId primitive.ObjectID, update bson.M) error {
	results := db.getPlaylistCollection().FindOneAndUpdate(ctx, map[string]interface{}{"_id": playlistId}, update)
	if results.Err() != nil {
		return results.Err()
	}
	return nil
}

func (db *MongoClient) DeletePlaylist(ctx context.Context, id primitive.ObjectID) error {
	results, err := db.getPlaylistCollection().DeleteOne(ctx, map[string]interface{}{"_id": id})
	if err != nil {
		return err
	} else if results.DeletedCount == 0 {
		return errors.New("no documents were deleted")
	}
	return nil
}

func (db *MongoClient) GetPlaylists(ctx context.Context, filters map[string]interface{}) ([]models.Playlist, error) {
	cursor, err := db.getPlaylistCollection().Find(ctx, filters)
	if err != nil {
		return nil, err
	}

	var results []models.Playlist
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (db *MongoClient) Ping(ctx context.Context) error {
	return db.Client.Ping(ctx, readpref.Primary())
}
