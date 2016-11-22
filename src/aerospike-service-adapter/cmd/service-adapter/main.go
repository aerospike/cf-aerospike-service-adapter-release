package main

import (
	"log"
	"os"

	"aerospike-service-adapter/adapter"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

func main() {
	stderrLogger := log.New(os.Stderr, "aerospike ", log.LstdFlags)
	manifestGenerator := &adapter.ManifestGenerator{
		StderrLogger: stderrLogger,
	}
	binder := &adapter.Binder{
		CommandRunner:       adapter.ExternalCommandRunner{},
		StderrLogger:        stderrLogger,
	}
	serviceadapter.HandleCommandLineInvocation(os.Args, manifestGenerator, binder, &adapter.DashboardUrlGenerator{})
}