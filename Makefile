SHELL := /bin/bash
UNAME_S := $(shell uname -s)

SED := sed
GREP := grep
ifeq ($(UNAME_S),Darwin)
	SED = gsed
	GREP = ggrep
endif


TEST_COMPOSE_FILE   = test/docker-compose.yml
DOCKER_IMAGE        = quay.io/evertrust/horizon:2.7.10
TEST_IMAGE          = quay.io/evertrust/ci-base:20241128
NETWORK             = bridge

#============= setup_tests

build:
	docker buildx build --platform linux/amd64 \
	 --network host -t ${DOCKER_IMAGE} . ; \

stack-run:
	@export DOCKER_IMAGE=${DOCKER_IMAGE}; \
	RESULT=0; \
	docker compose -f ${TEST_COMPOSE_FILE} up --detach --wait; \
	if [ $$? -ne 0 ]; then RESULT=1; fi; \
	docker save ${DOCKER_IMAGE} > ./backend.tar; \
	exit $$RESULT

stack-clean:
	@export DOCKER_IMAGE=${DOCKER_IMAGE}; \
	docker compose -f ${TEST_COMPOSE_FILE} down --rmi local --volumes

get-admin-password:
	@export DOCKER_IMAGE=${DOCKER_IMAGE}; \
	docker compose -f ${TEST_COMPOSE_FILE} exec horizon sh -c 'cat /tmp/tmp.*/adminPassword'

setup:
	${MAKE} build; \
	${MAKE} stack-run

#==== test

test-init:
	@if [ -z "$$ADMIN_PASSWORD" ]; then export ADMIN_PASSWORD=$$(${MAKE} get-admin-password --no-print-directory); fi; \
	IS_NEW_CONTAINER=0; \
	if [ -z "$$TEST_CONTAINER_ID" ]; then \
		IS_NEW_CONTAINER=1; \
		export TEST_CONTAINER_ID=$$(docker run -e LCC_CONFIG_FILE_PATH="/data/lcc/conf/ci.json" -e ADMIN_PASSWORD="$$ADMIN_PASSWORD" -e HORIZON_URL_HTTP="horizon:9000" -e HORIZON_URL_HTTPS="nginx" -d -v "$$(pwd)/test:/data/" -v /var/run/docker.sock:/var/run/docker.sock  ${TEST_IMAGE} -- sleep infinity); \
		docker network connect horizon_horizon $$TEST_CONTAINER_ID --alias horizon-stack.evertrust.fr ; \
	fi; \
	RESULT=0; \
	docker exec "$$TEST_CONTAINER_ID" bash -c "cd /data/Evertrust-Horizon-api-init && bru run \
	  --env-var X-API-KEY=\"$$ADMIN_PASSWORD\" \
	  --env-var horizonUrl=http://horizon:9000  \
	  --env Cicd"; \
	if [ $$? -ne 0 ]; then RESULT=1; fi; \
	if [ $$IS_NEW_CONTAINER -eq 1 ]; then \
		docker rm --force $$TEST_CONTAINER_ID; \
	fi; \
	exit $$RESULT

go-test:
	TF_ACC=1 go test ./... -v

#============= cleanup tests
cleanup-tests:
	RESULT=0; \
	if [ $$? -ne 0 ]; then RESULT=1; fi; \
	${MAKE} stack-clean; \
	exit $$RESULT
