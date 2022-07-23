include .env

SHELL := /bin/sh
.DEFAULT_GOAL := default
MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
CURRENT_DIR := $(patsubst %/,%,$(dir $(MAKEFILE_PATH)))

.PHONY: build
build:
	@docker build -t kpeu3i/zpcheckin .

.PHONY: push
push:
	@docker login -u "${DOCKER_LOGIN}" -p "${DOCKER_PASSWORD}"
	@docker image tag kpeu3i/zpcheckin kpeu3i/zpcheckin:${t}
	@docker image tag kpeu3i/zpcheckin kpeu3i/zpcheckin:latest
	@docker push kpeu3i/zpcheckin:${t}
	@docker push kpeu3i/zpcheckin:latest
	@docker logout 2>/dev/null || true

.PHONY: release
release: build push