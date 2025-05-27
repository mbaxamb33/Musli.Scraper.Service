package db

import (
	"database/sql/driver"
	"fmt"
)

type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCanceled   JobStatus = "canceled"
)

func (e *JobStatus) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = JobStatus(s)
	case string:
		*e = JobStatus(s)
	default:
		return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type %T", src, e)
	}
	return nil
}

func (e JobStatus) Value() (driver.Value, error) {
	return string(e), nil
}

func (e JobStatus) Valid() bool {
	switch e {
	case JobStatusPending, JobStatusProcessing, JobStatusCompleted, JobStatusFailed, JobStatusCanceled:
		return true
	}
	return false
}

func (e JobStatus) String() string {
	return string(e)
}
