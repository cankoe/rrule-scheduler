package models

import "time"

type Schedule struct {
	ID          string            `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string            `bson:"name" json:"name"`
	RRule       string            `bson:"rrule" json:"rrule"`
	CallbackURL string            `bson:"callback_url" json:"callback_url"`
	Method      string            `bson:"method,omitempty" json:"method,omitempty"`
	Headers     map[string]string `bson:"headers,omitempty" json:"headers,omitempty"`
	Body        string            `bson:"body,omitempty" json:"body,omitempty"`
	CreatedAt   time.Time         `bson:"created_at,omitempty" json:"created_at,omitempty"`
}
