package models

import "time"

type Schedule struct {
	ID          string    `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string    `bson:"name" json:"name"`
	RRule       string    `bson:"rrule" json:"rrule"`
	CallbackURL string    `bson:"callback_url" json:"callback_url"`
	CreatedAt   time.Time `bson:"created_at,omitempty" json:"created_at,omitempty"`
}
