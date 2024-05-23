package main

import (
	"flag"

	client "github.com/wbyatt/rolodex/client"
	server "github.com/wbyatt/rolodex/server"
)

var (
	port     = flag.Int("port", 1337, "The Rolodex API port")
	loadTest = flag.Bool(
		"load-test",
		false,
		"Run the Rolodex client load test")
)

func main() {
	flag.Parse()

	if *loadTest {
		client.Main()
	} else {
		server.Main(port)
	}
}
