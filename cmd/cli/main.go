package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/cubny/fare"
	"log"
	"os"
	"os/signal"
)

const maxSpeed = 100

func main() {
	infile := flag.String("input", "", "input csv file path")
	outfile := flag.String("output", "fares.csv", "output csv file path")
	concurrency := flag.Int("c", 5, "concurrent workers")
	flag.Parse()

	in, err := os.Open(*infile)
	if err != nil {
		log.Fatalf("open input file: %s\n", err)
	}

	out, err := os.Create(*outfile)
	if err != nil {
		log.Fatalf("open output in: %s\n", err)
	}

	defer func() {
		if err := in.Close(); err != nil {
			log.Fatalf("close input file: %s\n", err)
		}
		if err := out.Close(); err != nil {
			log.Fatalf("close output file: %s\n", err)
		}
	}()

	config := &fare.Config{
		MaxSpeed:    maxSpeed,
		Concurrency: *concurrency,
	}

	estimator, err := fare.NewEstimator(in, out, config)
	if err != nil {
		log.Fatalf("NewEstimator: %s\n", err)
	}

	ctx, stop := context.WithCancel(context.Background())

	exit := make(chan struct{})

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint
		stop()
	}()

	go func() {
		if err := estimator.Run(ctx); err != nil {
			log.Fatalf("estimator: %s\n", err)
		}
		exit <- struct{}{}
	}()

	<-exit
	fmt.Printf("output is written to %s\n", *outfile)
	fmt.Println("exit.")
}
