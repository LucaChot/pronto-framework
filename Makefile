DOCKER_USERNAME=lucachot
GO_FLAGS=

ifdef RACE
	GO_FLAGS += -race
endif

TAG ?= latest

ifeq ($(SCHED), CTL)
BINARY = main.go
BINARY += msg
OUTPUT = bin/sched
DOCKER_NAME = sched-framework
endif

all: build push

msg: message/message.proto
	protoc --go_out=. --go_opt=paths=source_relative \
	--go-grpc_out=. --go-grpc_opt=paths=source_relative $<

compile: ${BINARY}
	go build ${GO_FLAGS} -o ${OUTPUT} $<

build: compile
	docker build -t ${DOCKER_USERNAME}/${DOCKER_NAME}:${TAG} -f ${DOCKER_NAME}-docker .

push:
	docker push ${DOCKER_USERNAME}/${DOCKER_NAME}:${TAG}
