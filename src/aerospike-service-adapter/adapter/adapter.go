package adapter

import (
	"log"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

type ManifestGenerator struct {
	StderrLogger *log.Logger
}

type Binder struct {
	CommandRunner
	StderrLogger *log.Logger
}

var InstanceGroupMapper = serviceadapter.GenerateInstanceGroupsWithNoProperties