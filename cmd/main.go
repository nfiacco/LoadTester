package main

import (
	"flag"
	"fmt"
	"os"

	"nfiacco/loadtester/internal/runner"
)

func main() {
	fs := flag.NewFlagSet("loadtest", flag.ExitOnError)

	opts := runner.LoadTestArgs{}

	version := fs.Bool("version", false, "Print version and exit")
	fs.DurationVar(&opts.Duration, "duration", 0, "Duration of the test [0 = forever]")
	fs.Uint64Var(&opts.Qps, "qps", 100, "Queries per second")
	fs.Uint64Var(&opts.Workers, "workers", 100, "Number of initial workers")
	fs.Uint64Var(&opts.MaxWorkers, "max_workers", 100, "Max number of workers")
	fs.BoolVar(&opts.AutoScale, "autoscale", true, "Whether to automatically scale the number of workers")
	fs.Uint64Var(&opts.Timeout, "timeout", 30, "Timeout to wait for each request in seconds")
	fs.StringVar(&opts.Method, "method", "GET", "HTTP method to use")
	fs.StringVar(&opts.OutputFile, "output_file", "stdout", "Output file to write results to. Defaults to \"stdout\"")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: loadtest [flags] target")
		fs.PrintDefaults()
	}

	fs.Parse(os.Args[1:])

	if *version {
		fmt.Println("Version: 1.0")
		return
	}

	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(1)
	}

	target := fs.Arg(0)

	r := runner.NewRunner(target, opts)
	err := r.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
