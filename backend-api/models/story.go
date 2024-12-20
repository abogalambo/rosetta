package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Story struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Title       string             `bson:"title"`
	Segments    []Segment          `bson:"segments"`
	CreatedAt   time.Time          `bson:"created_at"`
	IsPublished bool               `bson:"is_published"`
}

type Segment struct {
	ID     primitive.ObjectID `bson:"_id,omitempty"`
	Audio  *Audio             `bson:"audio,omitempty"`
	Image  *Image             `bson:"image,omitempty"`
	Script *Script            `bson:"script,omitempty"`
}

type Audio struct {
	Url string `bson:"url,omitempty"`
}

type Image struct {
	Url string `bson:"url,omitempty"`
}

type Script struct {
	Text string `bson:"text"`
}
