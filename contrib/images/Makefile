all: epixd-env

epixd-env: epixd-rmi
	docker build --tag cosmos/epixd -f epixd-env/Dockerfile \
		$(shell git rev-parse --show-toplevel)

epixd-rmi:
	docker rmi cosmos/epixd 2>/dev/null; true

.PHONY: all epixd-env epixd-rmi
