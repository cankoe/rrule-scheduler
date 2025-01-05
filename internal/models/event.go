package models

import "time"

type StatusEntry struct {
	Time    time.Time `bson:"time"`
	Status  string    `bson:"status"`
	Message string    `bson:"message"`
}

type Event struct {
	ID         string        `bson:"_id,omitempty"`
	ScheduleID string        `bson:"schedule_id"`
	RunTime    time.Time     `bson:"run_time"`
	Status     []StatusEntry `bson:"status"`
	CreatedAt  time.Time     `bson:"created_at,omitempty"`
}
