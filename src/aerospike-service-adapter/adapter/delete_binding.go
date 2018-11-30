package adapter

import (

	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

func (b *Binder) DeleteBinding(
	bindingId string,
	boshVMs bosh.BoshVMs,
	manifest bosh.BoshManifest,
	requestParameters serviceadapter.RequestParameters,
	secrets serviceadapter.ManifestSecrets) error {
	
	// Add any cleanup code here...

	return nil
}