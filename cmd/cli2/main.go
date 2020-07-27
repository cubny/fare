package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"sync"

	fairestimation "github.com/cubny/fair-estimation"
)

type Line []string

func main() {
	cpuProfile, _ := os.Create("cpuprofile")
	memProfile, _ := os.Create("memprofile")
	pprof.StartCPUProfile(cpuProfile)

	in, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("open input file: %s", err)
	}

	concurrency, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatalf("concurrency: %s", err)
	}

	out, err := os.Create(os.Args[3])
	if err != nil {
		log.Fatalf("open output in: %s", err)
	}

	defer func() {
		if err := in.Close(); err != nil {
			log.Printf("close input file: %s", err)
		}
		if err := out.Close(); err != nil {
			log.Printf("close output file: %s", err)
		}
	}()

	ctx, stop := context.WithCancel(context.Background())
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)

	go func() {
		_, err = EstimateAll(ctx, concurrency, in, out)
		if err != nil {
			log.Printf("got error: %s\n", err)
		}
		sigint <- os.Interrupt
	}()
	<-sigint
	log.Printf("Caught ctrl-c...")
	stop()
	fmt.Println("writing profiles")
	pprof.StopCPUProfile()
	pprof.WriteHeapProfile(memProfile)
	fmt.Println("exit")

}

type RideResult struct {
	id    int
	price fairestimation.Price
	err   error
}

func EstimateAll(ctx context.Context, concurrency int, r io.Reader, w io.WriteCloser) (map[int]fairestimation.Price, error) {
	output := csv.NewWriter(w)

	linec, errc := GenLines(ctx, r)
	rides, errc1 := AggregateRides(ctx, linec)

	var wg sync.WaitGroup
	fairc := make(chan RideResult)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			EstimateRide(ctx, rides, fairc)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(fairc)
	}()

	fairs := make(map[int]fairestimation.Price)

	for f := range fairc {
		select {
		case <-ctx.Done():
			return nil, errors.New("EstimateAll canceled")
		default:
			//if _, ok := fairs[f.id]; ok {
			//	log.Println("price already exists")
			//	continue
			//}
			if f.err != nil {
				log.Println("price got error", f.err)
				continue
			}
			//fairs[f.id] = f.price
			fairEstimate := strconv.FormatFloat(float64(f.price), 'f', 2, 64)
			rideId := strconv.Itoa(f.id)
			record := []string{rideId, fairEstimate}
			err := output.Write(record)
			if err != nil {
				return nil, err
			}
		}
	}

	output.Flush()
	if err := output.Error(); err != nil {
		return nil, err
	}

	errm := mergeErrors(ctx, errc, errc1)
	for err := range errm {
		if err != nil {
			return nil, err
		}
	}

	return fairs, nil
}

func GenLines(ctx context.Context, reader io.Reader) (<-chan Line, <-chan error) {
	linec := make(chan Line, 5)
	errc := make(chan error)
	go func() {
		defer func() {
			close(linec)
			close(errc)
		}()
		in := csv.NewReader(reader)
		for {
			record, err := in.Read()
			if err == io.EOF {
				return
			}
			if err != nil {
				errc <- err
				return
			}
			select {
			case linec <- record:
			case <-ctx.Done():
				fmt.Println("reading lines canceled")
				return
			}
		}
	}()

	return linec, errc
}

func AggregateRides(ctx context.Context, linec <-chan Line) (<-chan []Line, <-chan error) {
	rideLinec := make(chan []Line)
	errc := make(chan error)

	go func() {
		var RideLines []Line
		lastRideID := ""
		defer func() {
			// flush the last aggregated lines
			if len(RideLines) > 0 {
				select {
				case <-ctx.Done():
					errc <- errors.New("splitting rides canceled")
				case rideLinec <- RideLines:
					//fmt.Println(RideLines[0][0], len(RideLines))
				}
			}
			close(rideLinec)
			close(errc)
		}()
		for line := range linec {
			select {
			case <-ctx.Done():
				errc <- errors.New("splitting rides canceled")
				return
			default:
				if line[0] == lastRideID {
					RideLines = append(RideLines, line)
					lastRideID = line[0]
					continue
				}
				if len(RideLines) > 0 {
					rideLinec <- RideLines
				}
				RideLines, lastRideID = []Line{line}, line[0]
			}
		}
	}()
	return rideLinec, errc
}

type SegmentPriceResult struct {
	RideID int
	price  fairestimation.Price
	err    error
}

type SegmentResult struct {
	segment fairestimation.Segment
	err     error
}

func GenSegments(ctx context.Context, positionResultc <-chan PositionResult) <-chan SegmentResult {
	segmentc := make(chan SegmentResult)
	go func() {
		defer close(segmentc)
		lastp := <-positionResultc
		for p := range positionResultc {
			segment, err := func(p PositionResult) (fairestimation.Segment, error) {
				defer func() {
					lastp = p
				}()
				if p.err != nil {
					return fairestimation.Segment{}, p.err
				}
				seg, err := fairestimation.NewSegment(lastp.position, p.position)
				if err != nil {
					return fairestimation.Segment{}, err
				}
				return seg, nil
			}(p)
			select {
			case segmentc <- SegmentResult{segment: segment, err: err}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return segmentc
}

func EstimateSegment(ctx context.Context, segmentResultc <-chan SegmentResult) <-chan SegmentPriceResult {
	segmentFairc := make(chan SegmentPriceResult)
	go func() {
		defer close(segmentFairc)
		for segmentResult := range segmentResultc {
			if segmentResult.err != nil {
				// filter all erogenous segments
				continue
			}
			select {
			case segmentFairc <- SegmentPriceResult{RideID: segmentResult.segment.RideID(), price: segmentResult.segment.FairEst(), err: nil}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return segmentFairc
}

func EstimateRide(ctx context.Context, ridec <-chan []Line, fairc chan<- RideResult) {
	for lines := range ridec {
		positionResultc := GenPositions(ctx, lines)
		segmentResultc := GenSegments(ctx, positionResultc)
		segmentFairc := EstimateSegment(ctx, segmentResultc)
		totalPrice := fairestimation.Price(0)
		var rideID int
		for sr := range segmentFairc {
			select {
			case <-ctx.Done():
				return
			default:
				if sr.err != nil {
					//fmt.Println(sr.err)
					continue
				}
				rideID = sr.RideID
				totalPrice += sr.price
			}
		}
		//if err := <-errc; err != nil {
		//	fairc <- RideResult{err: err}
		//	return
		//}

		select {
		case fairc <- RideResult{id: rideID, price: totalPrice}:
		case <-ctx.Done():
			return
		}
	}
}

type PositionResult struct {
	position fairestimation.Position
	err      error
}

func GenPositions(ctx context.Context, lines []Line) <-chan PositionResult {
	resultc := make(chan PositionResult)
	go func() {
		defer close(resultc)
		for _, r := range lines {
			pos, err := fairestimation.NewPosition(r[0], r[1], r[2], r[3])
			select {
			case resultc <- PositionResult{position: pos, err: err}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return resultc
}

func mergeErrors(ctx context.Context, errs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	outc := make(chan error, len(errs))
	output := func(errc <-chan error) {
		defer wg.Done()
		for e := range errc {
			select {
			case outc <- e:
			case <-ctx.Done():
				return
			}
		}
	}

	wg.Add(len(errs))
	for _, errc := range errs {
		go output(errc)
	}

	go func() {
		wg.Wait()
		close(outc)
	}()

	return outc
}
