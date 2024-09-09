package utils

import (
	"time"

	"github.com/gofiber/fiber/v2/log"
)

// LogDuration logs the duration of a function execution
func LogDuration(functionName string, start time.Time, args ...interface{}) {
	duration := time.Since(start)
	if len(args) > 0 {
		log.Debugf("%s took %v with args %v", functionName, duration, args)
	} else {
		log.Debugf("%s took %v", functionName, duration)
	}
}
