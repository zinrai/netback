package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type Result struct {
	Device   *Device
	Duration time.Duration
	Err      error
}

func runBackups(routerdb *RouterDB, modelFile *ModelFile, outputDir string, workers int) []Result {
	// Not appended from the goroutines: one slot per device keeps the results
	// in routerdb order without a lock, so what is reported does not vary
	// between runs.
	results := make([]Result, len(routerdb.Devices))

	var wg sync.WaitGroup

	slots := make(chan struct{}, workers)

	for i := range routerdb.Devices {
		device := &routerdb.Devices[i]

		result := &results[i]
		result.Device = device

		model, ok := modelFile.Models[device.Model]
		if !ok {
			result.Err = fmt.Errorf("model %q not found", device.Model)
			log.Printf("%s: failed - %v", device.Name, result.Err)
			continue
		}

		wg.Add(1)

		go func() {
			defer wg.Done()
			backupOne(slots, result, model, outputDir)
		}()
	}

	wg.Wait()

	return results
}

func backupOne(slots chan struct{}, result *Result, model *Model, outputDir string) {
	slots <- struct{}{}
	defer func() { <-slots }()

	device := result.Device

	// Not started before the slot is taken: that would measure how long the
	// device waited behind the others rather than how long the device took.
	start := time.Now()
	result.Err = backupDevice(device, model, outputDir)
	result.Duration = time.Since(start)

	if result.Err != nil {
		log.Printf("%s: failed - %v", device.Name, result.Err)
		return
	}

	log.Printf("%s: ok", device.Name)
}

func backupDevice(device *Device, model *Model, outputDir string) error {
	comments, commands, err := collectOutput(device, model)
	if err != nil {
		return err
	}

	content := renderOutput(comments, commands, model)

	// Not written out: replacing a good backup with an empty file loses the
	// only copy of the configuration.
	if content == "" {
		return fmt.Errorf("no output collected")
	}

	return writeConfig(outputDir, device.Group, device.Name, content)
}
