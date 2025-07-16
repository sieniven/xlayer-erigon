DOCKER_COMPOSE := docker compose -f docker-compose.yml
DOCKER_BRIDGE_DB := xlayer-bridge-db
DOCKER_BRIDGE_SERVICE := xlayer-bridge-service
DOCKER_BRIDGE_UI := xlayer-bridge-ui
DOCKER_APPROVE := xlayer-approve
DOCKER_SEQ := xlayer-seq
DOCKER_RPC := xlayer-rpc
DOCKER_L1_NETWORK := xlayer-mock-l1-network
DOCKER_POOL_DB := xlayer-pool-db
DOCKER_POOL_MANAGER := xlayer-pool-manager
DOCKER_DS := xlayer-ds
DOCKER_PROMETHEUS := prometheus
DOCKER_GRAFANA := grafana
DOCKER_MAINNET_SEQ := xlayer-mainnet-seq
DOCKER_MAINNET_RPC := xlayer-mainnet-rpc
DOCKER_KAFKA := xlayer-kafka

# Apollo
DOCKER_COMPOSE_APOLLO := docker compose -f docker-compose-apollo.yml
DOCKER_APOLLO_CONFIGSERVICE := apollo-configservice
DOCKER_APOLLO_ADMINSERVICE := apollo-adminservice
DOCKER_APOLLO_PORTAL := apollo-portal
DOCKER_APOLLO_MYSQL := mysql

RUN_DOCKER_BRIDGE_DB := $(DOCKER_COMPOSE) up -d $(DOCKER_BRIDGE_DB)
RUN_DOCKER_BRIDGE_SERVICE := $(DOCKER_COMPOSE) up -d $(DOCKER_BRIDGE_SERVICE)
RUN_DOCKER_BRIDGE_UI := $(DOCKER_COMPOSE) up -d $(DOCKER_BRIDGE_UI)
RUN_DOCKER_APPROVE := $(DOCKER_COMPOSE) up -d $(DOCKER_APPROVE)
RUN_DOCKER_SEQ := $(DOCKER_COMPOSE) up -d $(DOCKER_SEQ)
RUN_DOCKER_RPC := $(DOCKER_COMPOSE) up -d $(DOCKER_RPC)
RUN_DOCKER_MAINNET_SEQ := $(DOCKER_COMPOSE) up -d $(DOCKER_MAINNET_SEQ)
RUN_DOCKER_MAINNET_RPC := $(DOCKER_COMPOSE) up -d $(DOCKER_MAINNET_RPC)
RUN_DOCKER_L1_NETWORK := $(DOCKER_COMPOSE) up -d $(DOCKER_L1_NETWORK)
RUN_DOCKER_POOL_DB := $(DOCKER_COMPOSE) up -d $(DOCKER_POOL_DB)
RUN_DOCKER_POOL_MANAGER := $(DOCKER_COMPOSE) up -d $(DOCKER_POOL_MANAGER)
RUN_DOCKER_DS := $(DOCKER_COMPOSE) up -d $(DOCKER_DS)
RUN_DOCKER_PROMETHEUS := $(DOCKER_COMPOSE) up -d $(DOCKER_PROMETHEUS)
RUN_DOCKER_GRAFANA := $(DOCKER_COMPOSE) up -d $(DOCKER_GRAFANA)
RUN_DOCKER_KAFKA := $(DOCKER_COMPOSE) up -d $(DOCKER_KAFKA)

# Apollo
RUN_DOCKER_APOLLO_CONFIGSERVICE := $(DOCKER_COMPOSE_APOLLO) up -d $(DOCKER_APOLLO_CONFIGSERVICE)
RUN_DOCKER_APOLLO_ADMINSERVICE := $(DOCKER_COMPOSE_APOLLO) up -d $(DOCKER_APOLLO_ADMINSERVICE)
RUN_DOCKER_APOLLO_PORTAL := $(DOCKER_COMPOSE_APOLLO) up -d $(DOCKER_APOLLO_PORTAL)
RUN_DOCKER_APOLLO_MYSQL := $(DOCKER_COMPOSE_APOLLO) up -d $(DOCKER_APOLLO_MYSQL)
RUN_DOCKER_APOLLO_SEQ := $(DOCKER_COMPOSE_APOLLO) up -d $(DOCKER_SEQ)
RUN_DOCKER_APOLLO_RPC := $(DOCKER_COMPOSE_APOLLO) up -d $(DOCKER_RPC)


STOP_DOCKER_SEQ := $(DOCKER_COMPOSE) stop $(DOCKER_SEQ)
STOP_DOCKER_RPC := $(DOCKER_COMPOSE) stop $(DOCKER_RPC)
STOP := $(DOCKER_COMPOSE) down --remove-orphans; sleep 3; rm -rf data

INIT_BRIDGE_ACCOUNT := cast send -f 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534  --private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 --value 0.01ether 0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266 --legacy --rpc-url http://127.0.0.1:8123 --async
INIT_POLYGON_ZKEVM_BRIDGE_ACCOUNT := cast send -f 0x8943545177806ed17b9f23f0a21ee5948ecaa776  --private-key 0xbcdf20249abf0ed6d944c0288fad489e33f66b3960d9e6229c1cd214ed3bbe31 --value 0.01ether 0x3b59731225d8567ad89343a4669509fa91ba37ee --legacy --rpc-url http://127.0.0.1:8127 --async


.PHONY: build-docker
build-docker: ## Build cdk-erigoin for sequencer and rpc
	docker build -t cdk-erigon -f ../Dockerfile.local ../

.PHONY: run
run: stop build-docker
	$(DOCKER_COMPOSE) up -d xlayer-mock-l1-network
	$(DOCKER_COMPOSE) up -d xlayer-agg-db
	$(DOCKER_COMPOSE) up -d xlayer-bridge-db
	$(DOCKER_COMPOSE) up -d xlayer-pool-db
	$(DOCKER_COMPOSE) up -d xlayer-agglayer-prover
	$(DOCKER_COMPOSE) up -d xlayer-agglayer
	$(DOCKER_COMPOSE) up -d xlayer-bridge-redis
	$(DOCKER_COMPOSE) up -d kafka-zookeeper
	$(DOCKER_COMPOSE) up -d xlayer-bridge-coin-kafka

	sleep 3
	$(DOCKER_COMPOSE) up -d xlayer-approve
	$(DOCKER_COMPOSE) up -d xlayer-seq
	$(DOCKER_COMPOSE) up -d xlayer-pool-manager

	# sleep 5
	# $(DOCKER_COMPOSE) up -d xlayer-rpc

	sleep 10
	$(DOCKER_COMPOSE) up -d xlayer-bridge-service
	$(DOCKER_COMPOSE) up -d xlayer-bridge-ui
	$(INIT_BRIDGE_ACCOUNT)
	
	$(DOCKER_COMPOSE) up -d xlayer-cdk-node

.PHONY: all
all: run ## Runs a full node

.PHONY: stop
stop: ## Stops all services
	$(STOP)
