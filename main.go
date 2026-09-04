package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

type options struct {
	routerdb string
	model    string
	output   string
	workers  int
	timeout  time.Duration
	metrics  string
}

func main() {
	opts := parseOptions()

	routerdb, err := loadRouterDB(opts.routerdb, opts.timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading routerdb: %v\n", err)
		os.Exit(1)
	}

	modelFile, err := loadModelFile(opts.model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading model file: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(opts.output, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	results := runBackups(routerdb, modelFile, opts.output, opts.workers)

	failed := countFailures(results)
	log.Printf("Completed: %d success, %d failed", len(results)-failed, failed)

	// Not written before the summary: a metrics file that cannot be stored
	// would then hide how the run itself went.
	if opts.metrics != "" {
		if err := writeMetrics(opts.metrics, results); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing metrics: %v\n", err)
			os.Exit(1)
		}
	}

	if failed > 0 {
		os.Exit(1)
	}
}

func parseOptions() options {
	var (
		opts        options
		showVersion bool
	)

	flag.StringVar(&opts.routerdb, "routerdb", "", "Path to routerdb.yaml")
	flag.StringVar(&opts.model, "model", "", "Path to model.yaml")
	flag.StringVar(&opts.output, "output", "./configs", "Output directory")
	flag.IntVar(&opts.workers, "workers", 5, "Number of concurrent connections")
	flag.DurationVar(&opts.timeout, "timeout", 30*time.Second, "Default connection timeout")
	flag.StringVar(&opts.metrics, "metrics", "", "Path to write Prometheus metrics to when the run finishes")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.Parse()

	if showVersion {
		printVersion()
		os.Exit(0)
	}

	if opts.routerdb == "" || opts.model == "" {
		fmt.Fprintln(os.Stderr, "Usage: netback -routerdb <file> -model <file> [-output <dir>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	return opts
}

func countFailures(results []Result) int {
	failed := 0

	for _, r := range results {
		if r.Err != nil {
			failed++
		}
	}

	return failed
}
