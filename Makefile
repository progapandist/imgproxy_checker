SHELL := /bin/bash
.PHONY: copy-context drop-db

copy-context:
	find . -type f \( -iname "*.go" -o -iname "*.mod" \) -exec echo "==== {} ====" \; -exec cat {} \; | tee >(pbcopy)

drop-db:
	rm -rf ./image_data.db
