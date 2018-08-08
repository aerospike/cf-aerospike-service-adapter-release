package adapter

import (
	"errors"
	"fmt"
	"strings"	
	"time"
	"math/rand"
	"regexp"
	"strconv"
	"reflect"
	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

const OnlyStemcellAlias = "only-stemcell"
const AMCJobName = "aerospike-amc"
const RouteRegistrarJobName = "route_registrar"

var servicePlanParams = []string {
	"namespace_data_in_memory", "namespace_default_ttl", "namespace_filesize", "namespace_name", 
	"namespace_replication_factor", "namespace_size", "namespace_storage_type"}

var booleanParams = []string {
	"namespace_data_in_memory"}

var otherParams = []string {
	"server_route", "amc_route", "server_instances", "server_vm_type", "server_persistent_disk_type"}

var validParams = append(servicePlanParams, otherParams...)

// These parameters cannot be changed after initial configuration
var finalParams = []string {"namespace_name", "namespace_storage_type"}

func init() {
    rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func RandStringRunes(n int) string {
    b := make([]rune, n)
    for i := range b {
        b[i] = letterRunes[rand.Intn(len(letterRunes))]
    }
    return string(b)
}

// Map the Instance group of set of jobs running within the vm
func defaultDeploymentInstanceGroupsToJobs() map[string][]string {
	return map[string][]string{
		"Aerospike-Server":  []string{ "aerospike-server"	},
		"Aerospike-AMC":     []string{ "aerospike-amc", RouteRegistrarJobName},
	}
}

func (a *ManifestGenerator) GenerateManifest(serviceDeployment serviceadapter.ServiceDeployment,
	servicePlan serviceadapter.Plan,
	requestParams serviceadapter.RequestParameters,
	previousManifest *bosh.BoshManifest,
	previousPlan *serviceadapter.Plan,
) (bosh.BoshManifest, error) {

	featureKey := servicePlan.Properties["feature_key"].(string)
	natsDeploymentName := servicePlan.Properties["nats_deployment"].(string)

	copyOriginalManifestProperties(&servicePlan, previousManifest)
    aerospike_server_admin_username := "admin"
	aerospike_server_admin_password := RandStringRunes(20)
	aerospike_server_license_type := "enterprise"

    if originalPassword, ok := originalDbPassword(previousManifest); ok {
    	aerospike_server_admin_password = originalPassword
    }
 
	var releases []bosh.Release

	for _, serviceRelease := range serviceDeployment.Releases {
		releases = append(releases, bosh.Release{
			Name:    serviceRelease.Name,
			Version: serviceRelease.Version,
		})
	}
	
	servicePlanType := servicePlan.Properties["type"]

	a.StderrLogger.Printf("Service Releases: %+v\n", releases)
	a.StderrLogger.Printf("Service Plan Type: %s\n", servicePlanType)

	deploymentInstanceGroupsToJobs := defaultDeploymentInstanceGroupsToJobs()

	err := checkInstanceGroupsPresent([]string{
												"Aerospike-Server",
												"Aerospike-AMC",
												}, servicePlan.InstanceGroups)
	if err != nil {
		a.StderrLogger.Println(err.Error())
		return bosh.BoshManifest{}, errors.New("Contact your operator, service configuration issue occurred")
	}

	instanceGroups, err := InstanceGroupMapper(servicePlan.InstanceGroups, serviceDeployment.Releases, OnlyStemcellAlias, deploymentInstanceGroupsToJobs)
	if err != nil {
		a.StderrLogger.Println(err.Error())
		return bosh.BoshManifest{}, errors.New("Contact your operator, invalid instance groups")
	}

	arbitraryParameters := requestParams.ArbitraryParams()
	
	illegalArbParams := findIllegalArbitraryParams(arbitraryParameters)
	if len(illegalArbParams) != 0 {
		return bosh.BoshManifest{}, fmt.Errorf("unsupported parameter(s) : %s", strings.Join(illegalArbParams, ", "))
	}

	invalidArbParams := validateArbitraryParams(arbitraryParameters)
	if len(invalidArbParams) != 0 {
		return bosh.BoshManifest{}, fmt.Errorf("invalid parameter(s) : %s", strings.Join(invalidArbParams, ", "))
	}

	for _, ns_param := range servicePlanParams {
		if error_msg, ok := updateServicePlanProperty(&servicePlan, arbitraryParameters, previousManifest, ns_param ); !ok {
			return bosh.BoshManifest{}, errors.New(error_msg)
		}
	}

	service_map := map[string]interface{}{
		"db_user": aerospike_server_admin_username,
		"db_password": aerospike_server_admin_password,
		"license_type": aerospike_server_license_type,
		"feature_key": featureKey,
	}

	amc_service_map := map[string]interface{}{
		"db_user": aerospike_server_admin_username,
		"db_password": aerospike_server_admin_password,
		"license_type": aerospike_server_license_type,
		"amc_user": aerospike_server_admin_username,
		"amc_password": aerospike_server_admin_password,
	}

	service_network_map := map[string]interface{}{
		"service_port": 3000,
		"fabric_port": 3001,
		"info_port": 3003,
		"heartbeat_port": 3002,
		"heartbeat_interval": 150,
		"heartbeat_timeout": 10,
	}

	aerospike_serverInstanceGroup := &instanceGroups[0]

	if len(aerospike_serverInstanceGroup.Networks) != 1 {
		a.StderrLogger.Println(fmt.Sprintf("expected 1 network for %s, got %d", aerospike_serverInstanceGroup.Name, len(aerospike_serverInstanceGroup.Networks)))
		return bosh.BoshManifest{}, errors.New("")
	}

	aerospike_serverRoute := arbitraryParameters["server_route"]
	if aerospike_serverRoute == nil {
		aerospike_serverRoute = fmt.Sprintf("aerospike-server-%s", serviceDeployment.DeploymentName)
	}

	aerospike_amcRoute := arbitraryParameters["amc_route"]
	if aerospike_amcRoute == nil {
		aerospike_amcRoute = fmt.Sprintf("aerospike-amc-%s", serviceDeployment.DeploymentName)
		aerospike_amcRoute = strings.Replace(aerospike_amcRoute.(string), "service-instance_", "", 1)
	}

	aerospike_serverInstances := arbitraryParameters["server_instances"]
	if aerospike_serverInstances != nil {
		if floatval64, ok := aerospike_serverInstances.(float64); ok {
		    aerospike_serverInstanceGroup.Instances = int(floatval64)
		} else if intval, ok := aerospike_serverInstances.(int); ok {
		    aerospike_serverInstanceGroup.Instances = int(intval)
		} else if str, ok := aerospike_serverInstances.(string); ok {
			val, _ := strconv.ParseInt(str,10, 0)
			aerospike_serverInstanceGroup.Instances = int(val)
		}
	}

	aerospike_serverVMType := arbitraryParameters["server_vm_type"]
	if aerospike_serverVMType != nil {
		aerospike_serverInstanceGroup.VMType = aerospike_serverVMType.(string)
	} else if previousManifest != nil {
		if inst_group, ok := getInstanceGroup("Aerospike-Server", previousManifest.InstanceGroups); ok {
			aerospike_serverInstanceGroup.VMType = inst_group.VMType
		}	
	}

	aerospike_serverDiskType := arbitraryParameters["server_persistent_disk_type"]
	if aerospike_serverDiskType != nil {
		aerospike_serverInstanceGroup.PersistentDiskType = aerospike_serverDiskType.(string)
	} else if previousManifest != nil {
		if inst_group, ok := getInstanceGroup("Aerospike-Server", previousManifest.InstanceGroups); ok {
			aerospike_serverInstanceGroup.PersistentDiskType = inst_group.PersistentDiskType
		}	
	}
	if previousManifest != nil {
		if inst_group, ok := getInstanceGroup("Aerospike-Server", previousManifest.InstanceGroups); ok {
			previousInstanceCount := inst_group.Instances
			if (aerospike_serverInstanceGroup.Instances < previousInstanceCount) {
				a.StderrLogger.Println("Cannot reduce the size of the Aerospike cluster")
				return bosh.BoshManifest{}, errors.New("Cannot reduce the size of the Aerospike cluster")
			}
		}
	}

	updateServicePlan(&servicePlan, 
		servicePlanParams, 
		arbitraryParameters)

	aerospike_serverJob := &aerospike_serverInstanceGroup.Jobs[0]

	aerospike_serverJob.Properties = map[string]interface{}{
		"network": aerospike_serverInstanceGroup.Networks[0].Name,
		"service": service_map,
		"service_network": service_network_map,
	}
	for key, val := range servicePlan.Properties {
		aerospike_serverJob.Properties[key] = val
	}

	aerospike_amcInstanceGroup := &instanceGroups[1]

	if len(aerospike_amcInstanceGroup.Networks) != 1 {
		a.StderrLogger.Println(fmt.Sprintf("expected 1 network for %s, got %d", aerospike_amcInstanceGroup.Name, len(aerospike_serverInstanceGroup.Networks)))
		return bosh.BoshManifest{}, errors.New("")
	}

	aerospike_amcJob, _ :=  getJobFromInstanceGroup(AMCJobName, aerospike_amcInstanceGroup)

	aerospike_amcJob.Properties = map[string]interface{}{
		"network": aerospike_amcInstanceGroup.Networks[0].Name,
		"service": amc_service_map,
		"amc_listen_port": 8081,
		"amc_address": aerospike_amcRoute,
	}
	for key, val := range servicePlan.Properties {
		aerospike_amcJob.Properties[key] = val
	}

	route_registrarJob,  registrarFound := getJobFromInstanceGroup(RouteRegistrarJobName, aerospike_amcInstanceGroup)
	if registrarFound {
		addConsumesNatsToJob(route_registrarJob, natsDeploymentName)
		appsDomain := getAppsDomainFromPlan(servicePlan)
		route_registrarJob.Properties = buildHostsManifestPortion(aerospike_amcRoute.(string), appsDomain, 8081)

	}

	manifestProperties := map[string]interface{}{

	}	

	var updateBlock = bosh.Update{
		Canaries:        1,
		MaxInFlight:     1,
		CanaryWatchTime: "1000-3000000",
		UpdateWatchTime: "1000-3000000",
		Serial:          boolPointer(true),
	}

	if servicePlan.Update != nil {
		updateBlock = bosh.Update{
			Canaries:        servicePlan.Update.Canaries,
			MaxInFlight:     servicePlan.Update.MaxInFlight,
			CanaryWatchTime: servicePlan.Update.CanaryWatchTime,
			UpdateWatchTime: servicePlan.Update.UpdateWatchTime,
			Serial:          servicePlan.Update.Serial,
		}
	}

	generatedManifest := bosh.BoshManifest{
		Name:     serviceDeployment.DeploymentName,
		Releases: releases,
		Stemcells: []bosh.Stemcell{ {
				Alias:   OnlyStemcellAlias,
				OS:      serviceDeployment.Stemcell.OS,
				Version: serviceDeployment.Stemcell.Version, 
			}},
		InstanceGroups: instanceGroups,
		Properties:     manifestProperties,
		Update:         &updateBlock,
	}

	return generatedManifest, nil
}

func contains(s []string, e string) bool {
    for _, a := range s {
        if a == e {
            return true
        }
    }
    return false
}

func updateServicePlanProperty(servicePlan *serviceadapter.Plan, arbitraryParameters map[string]interface{}, previousManifest *bosh.BoshManifest, ns_param string) (string, bool){
	plan_param := servicePlan.Properties[ns_param]
	arb_param := arbitraryParameters[ns_param]
	orig_param, ok := getServerJobProperty(previousManifest, ns_param)
	if arb_param != nil && ok {
		if orig_param != arb_param && contains(finalParams, ns_param) {
			return fmt.Sprintf("Cannot change %s. Current value : %s", ns_param, orig_param), false
		}
		plan_param = arb_param
	} else if ok {
		plan_param = orig_param
	}
	servicePlan.Properties[ns_param] = plan_param

	if contains(booleanParams, ns_param) {
		if strVal, ok := plan_param.(string); ok {
			boolValue, err := strconv.ParseBool(strVal)
			if err != nil {
				return fmt.Sprintf("Cannot interpret %s as a boolean value", strVal), false
			}
			servicePlan.Properties[ns_param] = boolValue
		} else if boolVal, ok := plan_param.(bool); ok {
			servicePlan.Properties[ns_param] = boolVal
		} else {
			return fmt.Sprintf("Invalid value %s for boolean parameter %s", plan_param, ns_param), false
		}
	}

	return "", true
}

func findIllegalArbitraryParams(arbitraryParams map[string]interface{}) []string {
	var illegalParams []string
	for k, _ := range arbitraryParams {
		if ! contains( validParams, k) {
			illegalParams = append(illegalParams, k)
		}
	}
	return illegalParams
}

func validateArbitraryParams(arbitraryParams map[string]interface{}) []string {
	var invalidParams []string
	for key, _ := range arbitraryParams {
		switch key {
		case "namespace_size", "namespace_filesize":
			if  matched, _ := regexp.MatchString("[0-9]+[KMGTP]", arbitraryParams[key].(string)); !matched {
				invalidParams = append(invalidParams, fmt.Sprintf("invalid argument for %s: %s", key, arbitraryParams[key]))
			}
		case "namespace_storage_type":
			if  arbitraryParams[key].(string) != "device" && arbitraryParams[key].(string) != "memory" {
				invalidParams = append(invalidParams, fmt.Sprintf("invalid argument for %s: %s", key, arbitraryParams[key]))
			}
		}
	}
	return invalidParams
}

func copyOriginalManifestProperties(servicePlan *serviceadapter.Plan, previousManifest *bosh.BoshManifest) {
	if previousManifest != nil {
		// Only copy the properties from the previous manifest if the plan didn't change
		if previousManifest.Properties["type"] == servicePlan.Properties["type"] {
			if inst_group, ok := getInstanceGroup("Aerospike-Server", previousManifest.InstanceGroups); ok {
			 	if server_job, ok := getJobFromInstanceGroup("aerospike-server", inst_group); ok {
					for _, key := range servicePlanParams {
						if server_job.Properties[key] != nil {
							servicePlan.Properties[key] = server_job.Properties[key]
						}
					}
				}
			}
		}
	}
}

func getServerJobProperty(previousManifest *bosh.BoshManifest, propertyName string) (string, bool) {
	if previousManifest != nil {
		if inst_group, ok := getInstanceGroup("Aerospike-Server", previousManifest.InstanceGroups); ok {
		 	if server_job, ok := getJobFromInstanceGroup("aerospike-server", inst_group); ok {
				for k, v := range server_job.Properties { 
					if ( k == propertyName) {
						retVal := ""
						if strVal, ok := v.(string); ok {
		    				retVal = strVal
		    			} else if boolVal, ok := v.(bool); ok {
		    				if boolVal {
		    					retVal = "true"
		    				} else {
		    					retVal = "false"
		    				}
		    			} else if intVal, ok := v.(int); ok {
		    				retVal = strconv.Itoa(intVal)
		    			}else {
		    				return "", false
		    			}
		    			return retVal, true
					}
				}
			}
		}
	}

	return "", false
}

func updateServicePlan(servicePlan *serviceadapter.Plan, arb_param_list []string, arbitraryParameters map[string]interface{}) {
	for _, key := range arb_param_list {
		if arbitraryParameters[key] != nil {
			servicePlan.Properties[key] = arbitraryParameters[key]
		}
	}
}


func originalDbPassword(previousManifest *bosh.BoshManifest) (string, bool) {
	if previousManifest != nil {
		if inst_group, ok := getInstanceGroup("Aerospike-Server", previousManifest.InstanceGroups); ok {
		 	if server_job, ok := getJobFromInstanceGroup("aerospike-server", inst_group); ok {
		 		if service_map, ok := server_job.Properties["service"].(map[interface {}]interface{}); ok {
					for k, v := range service_map { 
						if ( k == "db_password") {
							return v.(string), true
						}
					}
				}
			}
		}
	}

	return "", false
}


func getInstanceGroup(name string, instanceGroups []bosh.InstanceGroup) (*bosh.InstanceGroup, bool) {
	for _, instanceGroup := range instanceGroups {
		if instanceGroup.Name == name {
			return &instanceGroup, true
		}
	}
	return &bosh.InstanceGroup{}, false
}

func getJobFromInstanceGroup(name string, instanceGroup *bosh.InstanceGroup) (*bosh.Job, bool) {
	for index, job := range instanceGroup.Jobs {
		if job.Name == name {
			return &instanceGroup.Jobs[index], true
		}
	}
	return &bosh.Job{}, false
}

func boolPointer(b bool) *bool {
	return &b
}

func checkInstanceGroupsPresent(names []string, instanceGroups []serviceadapter.InstanceGroup) error {
	var missingNames []string

	for _, name := range names {
		if !containsInstanceGroup(name, instanceGroups) {
			missingNames = append(missingNames, name)
		}
	}

	if len(missingNames) > 0 {
		return fmt.Errorf("Invalid instance group configuration: expected to find: '%s' in list: '%s'",
			strings.Join(missingNames, ", "),
			strings.Join(getInstanceGroupNames(instanceGroups), ", "))
	}
	return nil
}

func getInstanceGroupNames(instanceGroups []serviceadapter.InstanceGroup) []string {
	var instanceGroupNames []string
	for _, instanceGroup := range instanceGroups {
		instanceGroupNames = append(instanceGroupNames, instanceGroup.Name)
	}
	return instanceGroupNames
}

func containsInstanceGroup(name string, instanceGroups []serviceadapter.InstanceGroup) bool {
	for _, instanceGroup := range instanceGroups {
		if instanceGroup.Name == name {
			return true
		}
	}

	return false
}

func getAppsDomainFromPlan(servicePlan serviceadapter.Plan) string {

	cfMap := servicePlan.Properties["cf"]
	cfDomainRoute := ""
	if rec, ok := cfMap.(map[string]interface{}); ok {
		val, found := rec["app_domains"]
		if found {
			if (reflect.TypeOf(val).Kind() == reflect.String) {
				cfDomainRoute = val.(string)
			} else if (reflect.TypeOf(val).Kind() == reflect.Slice) {
				if rec, ok := val.([]interface{}); ok {
					cfDomainRoute = rec[0].(string)
				}	
			}
		}
	}
	return cfDomainRoute
}

func addConsumesNatsToJob(job *bosh.Job, deploymentName string) {
	*job = job.AddCrossDeploymentConsumesLink("nats", "nats", deploymentName)
}

func buildHostsManifestPortion(amcRoute string, appsDomain string, amcPort int) map[string]interface{}{
	// We want to build a structure looking like:
//	route_registrar.routes:
	// - name: my-service
	// registration_interval: 20s
	// port: 12345
	// tags:
	//   component: my-service
	//   env: production
	// uris:
	//   - my-service.system-domain.com
	//   - *.my-service.system-domain.com
	// health_check:
	//   name: my-service-health_check
	//   script_path: /path/to/script
	//   timeout: 5s
	amc_address := fmt.Sprintf("%s.%s", amcRoute, appsDomain)
	routes_info := map[string]interface{}{
		"name": amcRoute,
		"registration_interval": "20s",
		"port": amcPort,
		"uris": []string{amc_address},
	}

	route_registrar_info := map[string]interface{}{
		"route_registrar": map[string]interface{}{
			"routes": []map[string]interface{}{routes_info},
		},
	}
	return route_registrar_info
}
