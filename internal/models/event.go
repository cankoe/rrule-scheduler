package models

import "time"

// StatusEntry represents a single status update in the status array of an event.
type StatusEntry struct {
	Time    time.Time `bson:"time"`    // Timestamp of the status update
	Status  string    `bson:"status"`  // Status value (e.g., ready_queue, worker_queue, completed)
	Message string    `bson:"message"` // Additional information about the status update
}

// Event represents an individual execution of a schedule in the events collection.
type Event struct {
	ID         string        `bson:"_id,omitempty"`
	ScheduleID string        `bson:"schedule_id"`
	RunTime    time.Time     `bson:"run_time"`
	Status     []StatusEntry `bson:"status"`
	CreatedAt  time.Time     `bson:"created_at,omitempty"`
}
