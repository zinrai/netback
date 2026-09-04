package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// A device that failed has to come out as 0. Reporting it as a success leaves
// a green dashboard for a device with no current backup, which is the failure
// this option exists to surface.
func TestPrintMetricsReportsAFailedDeviceAsZero(t *testing.T) {
	results := []Result{
		{
			Device:   &Device{Name: "spine-01", Group: "dc-tokyo"},
			Duration: 2500 * time.Millisecond,
		},
		{
			Device:   &Device{Name: "leaf-01", Group: "dc-tokyo"},
			Duration: 30 * time.Second,
			Err:      errors.New("timeout waiting for prompt"),
		},
		{
			// Never contacted, so it has no duration. It still has to appear,
			// or the device goes missing from the dashboard instead of showing
			// up as failing.
			Device: &Device{Name: "spine-02", Group: "dc-osaka"},
			Err:    errors.New(`model "eos" not found`),
		},
	}

	var out strings.Builder
	printMetrics(&out, results)

	want := `# HELP netback_backup_success Config backup success status (1=success, 0=failure)
# TYPE netback_backup_success gauge
netback_backup_success{device="spine-01",group="dc-tokyo"} 1
netback_backup_success{device="leaf-01",group="dc-tokyo"} 0
netback_backup_success{device="spine-02",group="dc-osaka"} 0
# HELP netback_backup_duration_seconds Config backup duration in seconds
# TYPE netback_backup_duration_seconds gauge
netback_backup_duration_seconds{device="spine-01",group="dc-tokyo"} 2.500
netback_backup_duration_seconds{device="leaf-01",group="dc-tokyo"} 30.000
netback_backup_duration_seconds{device="spine-02",group="dc-osaka"} 0.000
`

	if got := out.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
