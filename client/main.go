package client

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/wbyatt/rolodex/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Main() {
	log.Printf("Spinning up client")

	conn, err := grpc.NewClient(
		"localhost:1337",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		log.Printf("Failed to connect: %v", err)
		panic(err)
	}

	defer conn.Close()

	client := api.NewRolodexClient(conn)

	durations := make([]float64, 10)
	rates := make([]float64, 10)

	for iteration := 0; iteration < 10; iteration++ {
		start := time.Now()

		iterations := 1000

		for i := 0; i < iterations; i++ {
			payload := &api.SetRequest{
				Key:   fmt.Sprint(i),
				Value: "5ca779f98e8ee77fd2b7159fd6dc6516e0ad735b92e1996c9192a1dc4aed1fd5",
			}

			ctx := context.Background()

			_, err := client.Set(ctx, payload)

			if err != nil {
				log.Printf("Failed to set: %v", err)
				panic(err)
			}
		}
		end := time.Now()

		duration := float64(end.Sub(start).Abs().Milliseconds())
		rate := float64(iterations) / float64(duration/1000)
		durations[iteration] = float64(duration)
		rates[iteration] = rate

		log.Printf("Finished iteration %v", iteration)
		log.Printf("\tTook: %v seconds", duration/1000)
		log.Printf("\tRate: %v qps", rate)
	}

	averageDuration := 0.0

	for _, duration := range durations {
		averageDuration += duration
	}

	averageDuration /= 10

	averageRate := 0.0

	for _, rate := range rates {
		averageRate += rate
	}

	averageRate /= float64(10)

	log.Printf("Completed.")
	log.Printf("\tAverage duration: %v seconds", averageDuration)
	log.Printf("\tAverage rate: %v qps", averageRate)

}
