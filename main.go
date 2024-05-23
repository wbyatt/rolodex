package main

import (
	"flag"

	server "github.com/wbyatt/rolodex/server"
)

var (
	port = flag.Int("port", 1337, "The Rolodex API port")
)

func main() {
	flag.Parse()

	server.Main(port)
}
