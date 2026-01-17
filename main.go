package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/zinrai/netback/config"
	"github.com/zinrai/netback/executor"
	"github.com/zinrai/netback/output"
)

func main() {
	var (
		routerdbPath  string
		modelPath     string
		outputDir     string
		workers       int
		defaultTimout time.Duration
	)

	flag.StringVar(&routerdbPath, "routerdb", "", "Path to routerdb.yaml")
	flag.StringVar(&modelPath, "model", "", "Path to model.yaml")
	flag.StringVar(&outputDir, "output", "./configs", "Output directory")
	flag.IntVar(&workers, "workers", 5, "Number of concurrent connections")
	flag.DurationVar(&defaultTimout, "timeout", 30*time.Second, "Default connection timeout")
	flag.Parse()

	if routerdbPath == "" || modelPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: netback -routerdb <file> -model <file> [-output <dir>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Load configurations
	routerdb, err := config.LoadRouterDB(routerdbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading routerdb: %v\n", err)
		os.Exit(1)
	}

	modelFile, err := config.LoadModelFile(modelPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading model file: %v\n", err)
		os.Exit(1)
	}

	// Prepare output
	writer := output.NewWriter(outputDir)
	if err := writer.EnsureDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Execute backups with concurrency control
	results := executeBackups(routerdb, modelFile, writer, workers)

	// Report results
	var success, failed int
	for _, r := range results {
		if r.Error != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", r.Device.Name, r.Error)
			failed++
		} else {
			fmt.Printf("OK   %s -> %s\n", r.Device.Name, writer.FilePath(r.Device.Name, r.Device.Group))
			success++
		}
	}

	fmt.Printf("\nCompleted: %d success, %d failed\n", success, failed)

	if failed > 0 {
		os.Exit(1)
	}
}

func executeBackups(
	routerdb *config.RouterDB,
	modelFile *config.ModelFile,
	writer *output.Writer,
	workers int,
) []*executor.Result {
	results := make([]*executor.Result, 0, len(routerdb.Devices))
	resultCh := make(chan *executor.Result, len(routerdb.Devices))

	// Semaphore for concurrency control
	sem := make(chan struct{}, workers)

	var wg sync.WaitGroup

	for i := range routerdb.Devices {
		device := &routerdb.Devices[i]

		model, ok := modelFile.Models[device.Model]
		if !ok {
			resultCh <- &executor.Result{
				Device: device,
				Error:  fmt.Errorf("model %q not found", device.Model),
			}
			continue
		}

		wg.Add(1)
		go func(d *config.Device, m *config.Model) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			result := executor.Execute(d, m)

			// Write output if successful
			if result.Error == nil {
				if err := writer.Write(d.Name, d.Group, result.Output); err != nil {
					result.Error = err
				}
			}

			resultCh <- result
		}(device, model)
	}

	// Wait for all goroutines and close channel
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results
	for r := range resultCh {
		results = append(results, r)
	}

	return results
}
