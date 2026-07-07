// Command geocheck prints the haversine distance (km) between two WGS-84
// points using the same implementation the matching logic relies on. It
// exists so scripts/smoke.sh can verify the distance math against known
// coordinates without needing an HTTP endpoint for it.
//
// Usage: go run ./cmd/geocheck <lat1> <lng1> <lat2> <lng2>
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"gandm/internal/geo"
)

func main() {
	if len(os.Args) != 5 {
		log.Fatal("usage: go run ./cmd/geocheck <lat1> <lng1> <lat2> <lng2>")
	}

	coords := make([]float64, 4)
	for i, arg := range os.Args[1:] {
		v, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			log.Fatalf("argument %d (%q) is not a number: %v", i+1, arg, err)
		}
		coords[i] = v
	}

	if !geo.ValidLatLng(coords[0], coords[1]) || !geo.ValidLatLng(coords[2], coords[3]) {
		log.Fatal("coordinates out of WGS-84 range")
	}

	fmt.Printf("%.3f\n", geo.HaversineKm(coords[0], coords[1], coords[2], coords[3]))
}
