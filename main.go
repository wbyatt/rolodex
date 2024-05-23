package main

import (
	"flag"

	client "github.com/wbyatt/rolodex/client"
	server "github.com/wbyatt/rolodex/server"
)

var (
	host     = flag.String("host", "localhost", "The Rolodex API host (or server target when running in client mode)")
	port     = flag.Int("port", 1337, "The Rolodex API port")
	loadTest = flag.Bool(
		"load-test",
		false,
		"Run the Rolodex client load test")
)

func main() {
	flag.Parse()

	if *loadTest {
		client.Main(port, host)
	} else {
		server.Main(port)
	}
}
