package main

import (
	"flag"

	client "github.com/wbyatt/rolodex/client"
	raft "github.com/wbyatt/rolodex/raft"
)

var (
	host         = flag.String("host", "localhost", "The Rolodex API host (or server target when running in client mode)")
	port         = flag.Int("port", 1337, "The Rolodex API port")
	raftPort     = flag.Int("raft-port", 7331, "The Raft consensus port")
	discoveryUrl = flag.String("discovery-url", "UNSET", "The Rolodex discovery URL")
	loadTest     = flag.Bool(
		"load-test",
		false,
		"Run the Rolodex client load test")
)

func main() {
	flag.Parse()

	if *loadTest {
		client.Main(port, host)
	} else {
		if *discoveryUrl == "UNSET" {
			panic("Missing discovery URL")
		}

		raft.Main(raftPort, discoveryUrl)
		// server.Main(port)
	}
}
