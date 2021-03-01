package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Track struct {
	ID			primitive.ObjectID	`json:"id" bson:"_id"`
	Name		string				`json:"name,omitempty" bson:"name,omitempty"`
	Artist		string				`json:"artist,omitempty" bson:"artist,omitempty,omitempty"`
	AlbumName	string				`json:"album,omitempty" bson:"album,omitempty"`
	AudioFileID	primitive.ObjectID	`json:"audioFile,omitempty" bson:"audioFile,omitempty"`
}

type Playlist struct {
	ID			primitive.ObjectID		`json:"id" bson:"_id"`
	Name		string					`json:"name" bson:"name"`
	Tracks		[]primitive.ObjectID	`json:"tracks,omitempty" bson:"tracks,omitempty"`
}
