
.PHONY: build-docker
build-docker:
	docker build -t cdk-erigon -f ../Dockerfile.local ../

.PHONY: run
run: build-docker
	docker-compose up -d xlayer-fep-prover

.PHONY: stop
stop:
	docker-compose rm -f xlayer-fep-prover 
