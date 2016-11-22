echo "Before running make sure the manifest.json contains updated json content"
echo "First capture the manifest in yml format from testManifest and then convert yaml to json"
echo "Use link to convert: http://codebeautify.org/yaml-to-json-xml-csv" 
echo ""
. ../go.env 
./create_test_binding.py create-binding
