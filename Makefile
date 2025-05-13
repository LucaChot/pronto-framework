DOCKER_USERNAME=lucachot
GO_FLAGS=

ifdef RACE
	GO_FLAGS += -race
endif

TAG ?= latest

ifeq ($(SCHED), CTL)
BINARY = main.go
OUTPUT = bin/sched
DOCKER_NAME = sched-framework
endif

all: build push

compile: ${BINARY}
	go build ${GO_FLAGS} -o ${OUTPUT} $<

build: compile
	docker build -t ${DOCKER_USERNAME}/${DOCKER_NAME}:${TAG} -f ${DOCKER_NAME}-docker .

push:
	docker push ${DOCKER_USERNAME}/${DOCKER_NAME}:${TAG}

