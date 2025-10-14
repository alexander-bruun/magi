package models

import (
	"database/sql"
	"time"
)

// ExistsChecker is a generic function to check if a record exists
func ExistsChecker(query string, args ...interface{}) (bool, error) {
	var exists int
	err := db.QueryRow(query, args...).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// TimestampPair holds creation and update timestamps
type TimestampPair struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewTimestamps creates a new timestamp pair with current time
func NewTimestamps() TimestampPair {
	now := time.Now()
	return TimestampPair{CreatedAt: now, UpdatedAt: now}
}

// UpdateTimestamp updates only the UpdatedAt field
func (t *TimestampPair) UpdateTimestamp() {
	t.UpdatedAt = time.Now()
}

// UnixTimestamps returns both timestamps as Unix seconds
func (t *TimestampPair) UnixTimestamps() (int64, int64) {
	return t.CreatedAt.Unix(), t.UpdatedAt.Unix()
}

// FromUnixTimestamps populates timestamps from Unix seconds
func (t *TimestampPair) FromUnixTimestamps(createdAt, updatedAt int64) {
	t.CreatedAt = time.Unix(createdAt, 0)
	t.UpdatedAt = time.Unix(updatedAt, 0)
}

// CountRecords returns count from a query
func CountRecords(query string, args ...interface{}) (int64, error) {
	var count int64
	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// DeleteRecord executes a delete query
func DeleteRecord(query string, args ...interface{}) error {
	_, err := db.Exec(query, args...)
	return err
}
