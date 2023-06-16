SHELL := /bin/bash
.PHONY: copy-context

copy-context:
	find . -type f \( -iname "*.go" -o -iname "*.mod" \) -exec echo "==== {} ====" \; -exec cat {} \; | tee >(pbcopy)
