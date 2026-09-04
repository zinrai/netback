package main

import (
	"fmt"
	"log"
	"sync"
)

func runBackups(routerdb *RouterDB, modelFile *ModelFile, outputDir string, workers int) int {
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed int
	)

	fail := func(name string, err error) {
		log.Printf("%s: failed - %v", name, err)
		mu.Lock()
		failed++
		mu.Unlock()
	}

	sem := make(chan struct{}, workers)

	for i := range routerdb.Devices {
		device := &routerdb.Devices[i]

		model, ok := modelFile.Models[device.Model]
		if !ok {
			fail(device.Name, fmt.Errorf("model %q not found", device.Model))
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if err := backupDevice(device, model, outputDir); err != nil {
				fail(device.Name, err)
				return
			}

			log.Printf("%s: ok", device.Name)
		}()
	}

	wg.Wait()

	return failed
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
