# Fare Estimator
_Fare_ is a command line program that estimates the fare of rides. 
A ride consists of a series of positions. The program, tries to make a  
path out of the given geo locations and calculates the fare of each ride
based on the business rules.
   
_Fare_ accepts an comma separated text file containing a list of tuples of the 
form `id_ride, lat, lng, timestamp`. The input file should be sorted 
by `id_ride` and `timestamp`, otherwise the program will not work correctly. 

The output is a comma separated text file, each line of the file is of the form
of `id_ride, fare_amount`. 
	

## How to Run it
```shell script
make build
./bin/fare -input data/paths.csv -output fares.csv -c 5
```

### Usage
````
Usage of fare:
  -c int
        concurrent workers (default 5)
  -input string
        input csv file path
  -output string
        output csv file path (default "fares.csv")
````


## Assumptions I made
- this program is designed for big input files (few GB)
- in calculation of segment's fare, I assumed it is timely short enough
that we don't need to break the Segment to two pieces like "before midnight" and
"during the midnight"
- the timestamp in the input files is always 10 digit epoch time
- based on the example data, the number of positions for each ride is less than few hundreds 


## Architecure
This solution is heavily based on pipeline patterns. It consists of two pipelines: main (estimator.go), ride (ride.go)
The main reads and writes records from and to csv files. the stream records are then group by a transformer stage into 
records of a rideID. After that, they are published them into a channel. the consumer of this channel, is a worker pool, 
which the workers spin up the next pipeline for calculating the total fare of each ride. up to this point the records are
passed as-is, which is a slice of string. It is only in the ride pipeline that they get converted to Position type and
Segment type.

                                                      +-----------+                                 
                                                      |           |                                 
                                                      |  csv.Read |                                 
                                                      |           |                                 
                                                      +-----|-----+                                 
                                                            |                                       
                                                    +---------------+                               
                                                    |               |                               
                                                    | Group by Ride |                               
                                                    |               |                               
                                                    +-------|-------+                               
                                                            |                                       
                             Worker Pool                    |                                       
                             ---------------------------------------------------------------        
                             |                   |                       |                  |       
                             |                   |                       |                  |       
                             |                   |                       |                  |       
                       +-----------+       +-----------+           +-----------+      +-----------+ 
                       | Generate  |       | Generate  |           | Generate  |      | Generate  | 
                       | Positions |       | Positions |           | Positions |      | Positions | 
                       +-----|-----+       +-----|-----+           +-----|-----+      +-----|-----+ 
                             |                   |                       |                  |       
                             |                   |                       |                  |       
                       +-----------+       +-----------+           +-----------+      +-----------+ 
                       | Reduce to |       | Reduce to |           | Reduce to |      | Reduce to | 
                       | Segments  |       | Segments  |           | Segments  |      | Segments  | 
                       +-----|-----+       +-----|-----+           +-----|-----+      +-----|-----+ 
                             |                   |                       |                  |       
                             |                   |                       |                  |       
                       +-----------+       +-----------+           +-----------+      +-----------+ 
                       | Fare of   |       | Fare of   |           | Fare of   |      | Fare of   | 
                       | Segment   |       | Segment   |           | Segment   |      | Segment   | 
                       +-----|-----+       +-----|-----+           +-----|-----+      +-----|-----+ 
                             |                   |                       |                  |       
                             |                   |                       |                  |       
                      +-------------+     +-------------+         +-------------+    +-------------+
                      |  Sum all    |     |  Sum all    |         |  Sum all    |    |  Sum all    |
                      |  Fares      |     |  Fares      |         |  Fares      |    |  Fares      |
                      +-------------+     +-------------+         +-------------+    +-------------+
                             |                   |                       |                  |       
                             |                   |                       |                  |       
                             v-------------------v-----------------------v------------------v       
                                                              |                                     
                                                              |                                     
                                                              |                                     
                                                              v                                     
                                                   +----------------------+                         
                                                   |                      |                         
                                                   |    Write to CSV      |                         
                                                   |                      |                         
                                                   +----------------------+                           


## Shortcomings of my solution
-  I used the csv package, assuming it makes good use of buffers. later when profiling I noticed that the 
most resource consuming part is IO read, I didn't have the time to test other solutions like `bufio.Scanner`.
- I sacrificed some performance improvements for better code readability, including using empty interface to make it 
possible to separate pipeline functions from business logic.
- Parsing positions is being done serially, but it should be faster to use fanout pattern for them as well
- creating Segment object is costly. especially since I used values everywhere. it can be problematic for huge input 
- test coverage is above 80%, however I didn't have time to write enough tests for estimate.go and ride.go
files, since the burden is on GC.
- Although I am sure queueing improves performance, but because of the issue above, I didn't get to find the right stage
to make use of it. After all, the bottleneck is not in the rest of the flow, so optimisation could only add to complexity.

## Making it scalable and some extra features
- using pools for objects such as Segment and Position would be beneficial. 
- reading from a file is not nice, anything other than files can lift the main burden off IO
- any type of database sql/nosql/newsql can help remove a few steps.
- make use of actual data pipelines 
- by using queue manager and multiple nodes for ingesting and processing more data concurrently, chunked either by size or time
- leverage caching of geo locations and the distance between two positions. even routes between them should be possible
