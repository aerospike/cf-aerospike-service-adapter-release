package adapter

import (
	"bytes"
	"os/exec"
	"errors"
	"fmt"
	"reflect"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

func (b *Binder) CreateBinding(
	bindingId string, boshVMs bosh.BoshVMs,
	manifest bosh.BoshManifest,
	requestParams serviceadapter.RequestParameters,
	secrets serviceadapter.ManifestSecrets,
	dnsInfo serviceadapter.DNSAddresses) (serviceadapter.Binding, error) {
	instanceGroups := manifest.InstanceGroups
	
    cfDomainRoute := ""
    servicePlanType := ""

	if (servicePlanType == "") {
		servicePlanType = instanceGroups[0].Jobs[0].Properties["type"].(string)
	}

	cfMap := instanceGroups[0].Jobs[0].Properties["cf"]
	
    if rec, ok := cfMap.(map[interface{}]interface{}); ok {
        for key, val := range rec {
        	keyStr := key.(string)
            if keyStr == "app_domains" {
            	if (reflect.TypeOf(val).Kind() == reflect.String) {
					cfDomainRoute = val.(string)
				} else if (reflect.TypeOf(val).Kind() == reflect.Slice) {
				    if rec, ok := val.([]interface{}); ok {
				    	cfDomainRoute = rec[0].(string)
				    }														
				}
				break
			} 
        }
    } 

	aerospike_serverUser, aerospike_serverPasswd := "", ""

	aerospike_serverHosts := boshVMs["Aerospike-Server"]
	if len(aerospike_serverHosts) == 0 {
		b.StderrLogger.Println("no VMs for instance group aerospike-server")
		return serviceadapter.Binding{}, errors.New("")
	}

	amc_serverHosts := boshVMs["Aerospike-AMC"]
	if len(amc_serverHosts) == 0 {
		b.StderrLogger.Println("no VMs for instance group amc_serverHosts")
		return serviceadapter.Binding{}, errors.New("")
	}

	aerospike_namespace := instanceGroups[0].Jobs[0].Properties["namespace_name"]

	serviceMap := instanceGroups[0].Jobs[0].Properties["service"]
	
    if rec, ok := serviceMap.(map[interface{}]interface{}); ok {
        for key, val := range rec {
        	keyStr := key.(string)
            if keyStr == "db_user" {
            	if (reflect.TypeOf(val).Kind() == reflect.String) {
					aerospike_serverUser = val.(string)
				} 													
			} else if  keyStr == "db_password" {
            	if (reflect.TypeOf(val).Kind() == reflect.String) {
					aerospike_serverPasswd = val.(string)
				} 													
			} 
        }
    }

    amc_address := fmt.Sprintf("https://%s.%s", instanceGroups[1].Jobs[0].Properties["amc_address"].(string), cfDomainRoute)
    serviceMap = instanceGroups[1].Jobs[0].Properties["service"]
	
    aerospike_servicePort := 2000
    serviceNetworkMap := instanceGroups[0].Jobs[0].Properties["service_network"]
	
    if rec, ok := serviceNetworkMap.(map[interface{}]interface{}); ok {
        for key, val := range rec {
        	keyStr := key.(string)
            if keyStr == "service_port" {
            	if (reflect.TypeOf(val).Kind() == reflect.Int) {
					aerospike_servicePort = val.(int)
				} 													
			}  
        }
    }

	arbitraryParameters := requestParams.ArbitraryParams()

	b.StderrLogger.Printf("[{'tile': {'short_name': 'aerospike-service-on-demand', 'label': 'Aerospike NoSQL Database'}, 'title': 'Aerospike', 'name': 'aerospike-service-adapter', 'short_name': 'aerospike', 'description': 'Aerospike NoSQL Database'}] CreateBinding with arbitraryParameters: %+v\n", arbitraryParameters)

	generatedBinding := serviceadapter.Binding{
		Credentials: map[string]interface{}{
			"service_type": servicePlanType,
			"aerospike_server_ips": aerospike_serverHosts,
			"user": aerospike_serverUser,
			"password": aerospike_serverPasswd,
			"aerospike_amc_ips": amc_serverHosts,
			"hostname": aerospike_serverHosts[0],
			"port": aerospike_servicePort,
			"namespace": aerospike_namespace,
			"amc_address": amc_address,
		},
	}

	b.StderrLogger.Printf("[{'tile': {'short_name': 'aerospike-service-on-demand', 'label': 'Aerospike NoSQL Database'}, 'title': 'Aerospike', 'name': 'aerospike-service-adapter', 'short_name': 'aerospike', 'description': 'Aerospike NoSQL Database'}] Generated Binding Creds:\n %+v\n", generatedBinding)
	return generatedBinding, nil
}

//go:generate counterfeiter -o fake_command_runner/fake_command_runner.go . CommandRunner
type CommandRunner interface {
	Run(name string, arg ...string) ([]byte, []byte, error)
}

type ExternalCommandRunner struct{}

func (c ExternalCommandRunner) Run(name string, arg ...string) ([]byte, []byte, error) {
	cmd := exec.Command(name, arg...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.Output()
	return stdout, stderr.Bytes(), err
}