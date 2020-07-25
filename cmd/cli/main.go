package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"

	fairestimation "github.com/cubny/fair-estimation"
)

func parseCSV(file io.Reader) ([]fairestimation.Position, error) {
	r := csv.NewReader(file)
	var positions []fairestimation.Position

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return positions, err
		}

		pos, err := fairestimation.NewPosition(record[0], record[1], record[2], record[3])
		if err != nil {
			return positions, err
		}

		positions = append(positions, pos)
	}
	return positions, nil
}

func main() {
	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	positions, err := parseCSV(file)
	if err != nil {
		log.Fatal(err)
	}

	var rides = make(map[int]fairestimation.Price)
	lastp := positions[0]
	for _, p := range positions {
		fmt.Printf("%s\n", p)
		seg, err := fairestimation.NewSegment(lastp, p)
		if err != nil {
			fmt.Println(err)
		}
		fair := seg.FairEst()
		fmt.Println(seg.String())
		if _, ok := rides[p.RideID]; !ok {
			rides[p.RideID] = 0
		}
		rides[p.RideID] += fair
		lastp = p
	}

	for id, f := range rides {
		fmt.Printf("ride %d, fair: %f\n", id, f)
	}
}
