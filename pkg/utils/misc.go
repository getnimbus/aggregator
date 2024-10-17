package utils

import (
	"fmt"
	"time"
)

func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)

	// calculate hours, minutes, and seconds
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60

	// build the formatted time string
	var formattedTime string
	if hours > 0 {
		formattedTime += fmt.Sprintf("%d hours, ", hours)
	}
	if minutes > 0 {
		formattedTime += fmt.Sprintf("%d minutes, ", minutes)
	}
	formattedTime += fmt.Sprintf("%d seconds", seconds)

	fmt.Printf("%s took: %s\n", name, formattedTime)
}
