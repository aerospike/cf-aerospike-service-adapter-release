#!/usr/bin/env python

from subprocess import call
from sys import argv

with open("./bosh-vms.json", "r") as myfile:
    bosh_vms = myfile.read().replace('\n', ' ').replace('\r', '')

with open("./manifest.json", "r") as myfile:
    manifest = myfile.read().replace('\n', ' ').replace('\r', '')

with open("./request.json", "r") as myfile:
    requestParams = myfile.read().replace('\n', ' ').replace('\r', '')
 
call(['go', 'run', '../src/aerospike-service-adapter/cmd/service-adapter/main.go', argv[1], '3412323i', bosh_vms, manifest, '{}' ]) 

