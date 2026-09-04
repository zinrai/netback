package main

import (
	"fmt"
	"io"
	"strings"
)

func writeMetrics(path string, results []Result) error {
	var out strings.Builder
	printMetrics(&out, results)

	return replaceFile(path, out.String())
}

func printMetrics(w io.Writer, results []Result) {
	fmt.Fprintln(w, "# HELP netback_backup_success Config backup success status (1=success, 0=failure)")
	fmt.Fprintln(w, "# TYPE netback_backup_success gauge")

	for _, r := range results {
		success := 0
		if r.Err == nil {
			success = 1
		}
		fmt.Fprintf(w, "netback_backup_success%s %d\n", labels(r.Device), success)
	}

	fmt.Fprintln(w, "# HELP netback_backup_duration_seconds Config backup duration in seconds")
	fmt.Fprintln(w, "# TYPE netback_backup_duration_seconds gauge")

	for _, r := range results {
		fmt.Fprintf(w, "netback_backup_duration_seconds%s %.3f\n", labels(r.Device), r.Duration.Seconds())
	}
}

func labels(device *Device) string {
	return fmt.Sprintf("{device=\"%s\",group=\"%s\"}", device.Name, device.Group)
}
