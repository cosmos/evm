all: evmd-env

evmd-env: evmd-rmi
	docker build --tag cosmos/evmd -f evmd-env/Dockerfile \
		$(shell git rev-parse --show-toplevel)

evmd-rmi:
	docker rmi cosmos/evmd 2>/dev/null; true

.PHONY: all evmd-env evmd-rmi
