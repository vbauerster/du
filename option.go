package main

type Options struct {
	Verbose bool `short:"v" long:"verbose" description:"Verbose progress messages"`
	K       bool `short:"k" description:"Display size in KiB"`
	G       bool `short:"g" description:"Display size in GiB"`
}
