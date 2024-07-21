package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringArray is a custom type that will be stored as a JSON string in the database
type StringArray []string

// Value implements the driver.Valuer interface, converting the StringArray to a JSON string
func (a StringArray) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan implements the sql.Scanner interface, converting a JSON string to a StringArray
func (a *StringArray) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, a)
}
