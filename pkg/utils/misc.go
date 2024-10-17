package utils

import (
	"fmt"
	"time"
)

func TimeTrack(start time.Time, name string) string {
	elapsed := time.Since(start)

	// calculate hours, minutes, seconds, and milliseconds
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	milliseconds := int(elapsed.Milliseconds()) % 1000

	var formattedTime string
	if hours > 0 {
		formattedTime += fmt.Sprintf("%d hours, ", hours)
	}
	if minutes > 0 {
		formattedTime += fmt.Sprintf("%d minutes, ", minutes)
	}
	if seconds > 0 {
		formattedTime += fmt.Sprintf("%d seconds, ", seconds)
	}
	formattedTime += fmt.Sprintf("%d ms", milliseconds)

	return fmt.Sprintf("%s took: %s", name, formattedTime)
}
