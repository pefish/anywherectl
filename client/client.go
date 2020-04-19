package client

import (
	"flag"
	"log"
	"os"
)

type Client struct {

}

func NewClient() *Client {
	return &Client{

	}
}

func (s *Client) DecorateFlagSet(flagSet *flag.FlagSet) {

}

func (s *Client) ParseFlagSet(flagSet *flag.FlagSet) {
	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}

func (c *Client) Start(flagSet *flag.FlagSet) error {

	//flagSet.String("tcp-address", opts.TCPAddress, "<addr>:<port> to listen on for TCP clients")
	//flagSet.String("http-address", opts.HTTPAddress, "<addr>:<port> to listen on for HTTP clients")
	//flagSet.String("broadcast-address", opts.BroadcastAddress, "address of this lookupd node, (default to the OS hostname)")
	//
	//flagSet.Duration("inactive-producer-timeout", opts.InactiveProducerTimeout, "duration of time a producer will remain in the active list since its last ping")
	//flagSet.Duration("tombstone-lifetime", opts.TombstoneLifetime, "duration of time a producer will remain tombstoned if registration remains")

	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func (s *Client) Stop() {

}
