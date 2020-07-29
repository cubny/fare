package haversine

import "math"

const earthRadius = float64(6371)

// Haversine calculates the great-circle distance between two points -- that is,
//the shortest distance over the earth’s surface – giving an ‘as-the-crow-flies’
//distance between the points (ignoring any hills they fly over, of course!).
// NOTE: snippet found at https://play.golang.org/p/MZVh5bRWqN
func Haversine(lonFrom float64, latFrom float64, lonTo float64, latTo float64) float64 {
	var deltaLat = (latTo - latFrom) * (math.Pi / 180)
	var deltaLon = (lonTo - lonFrom) * (math.Pi / 180)

	var a = math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(latFrom*(math.Pi/180))*math.Cos(latTo*(math.Pi/180))*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	var c = 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}
