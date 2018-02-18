pretty:
	bash -c 'for f in *.json; do echo $$f; jq . < $$f > $$f.tmp && mv $$f.tmp $$f; done'
