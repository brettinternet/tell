# Append `-notify` to any target: `make test-notify` runs `make test`, then notifies.
%-notify:
	@if $(MAKE) --no-print-directory $*; then \
		tell notify --type success --title "make $*" -- "Finished" || true; \
	else \
		tell notify --type failure --title "make $*" -- "Failed" || true; \
		exit 1; \
	fi
