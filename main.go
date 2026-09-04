package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	var (
		routerdbPath string
		modelPath    string
		outputDir    string
		workers      int
		timeout      time.Duration
		showVersion  bool
	)

	flag.StringVar(&routerdbPath, "routerdb", "", "Path to routerdb.yaml")
	flag.StringVar(&modelPath, "model", "", "Path to model.yaml")
	flag.StringVar(&outputDir, "output", "./configs", "Output directory")
	flag.IntVar(&workers, "workers", 5, "Number of concurrent connections")
	flag.DurationVar(&timeout, "timeout", 30*time.Second, "Default connection timeout")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.Parse()

	if showVersion {
		printVersion()
		os.Exit(0)
	}

	if routerdbPath == "" || modelPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: netback -routerdb <file> -model <file> [-output <dir>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	routerdb, err := loadRouterDB(routerdbPath, timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading routerdb: %v\n", err)
		os.Exit(1)
	}

	modelFile, err := loadModelFile(modelPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading model file: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	failed := runBackups(routerdb, modelFile, outputDir, workers)

	log.Printf("Completed: %d success, %d failed", len(routerdb.Devices)-failed, failed)

	if failed > 0 {
		os.Exit(1)
	}
}
