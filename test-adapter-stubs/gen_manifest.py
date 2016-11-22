#!/usr/bin/env python

from subprocess import call
from sys import argv
from os import system

with open("./deployment.json", "r") as myfile:
    bosh_info = myfile.read().replace('\n', ' ').replace('\r', '')

with open("./plan.json", "r") as myfile:
    plan = myfile.read().replace('\n', ' ').replace('\r', '')

with open("./request.json", "r") as myfile:
    requestParams = myfile.read().replace('\n', ' ').replace('\r', '')

with open("./prevManifest.yml", "r") as prevManifest:
	prev_manifest = prevManifest.read()

#call(['go', 'run', '../src/aerospike-service-adapter/cmd/service-adapter/main.go', argv[1], bosh_info, plan, requestParams, '---', '{}'])
call(['go', 'run', '../src/aerospike-service-adapter/cmd/service-adapter/main.go', argv[1], bosh_info, plan, requestParams, prev_manifest, '{}'])

