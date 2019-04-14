IMAGE_REPO=andrewsykim/kube-ovs
IMAGE_TAG=v1

.PHONY: compile
compile:
	docker run \
	  -v $(PWD):/go/src/github.com/kube-ovs/kube-ovs \
	  -w /go/src/github.com/kube-ovs/kube-ovs \
	  -e GOOS=linux -e GOFLAGS=-mod=vendor -e GO111MODULE=on \
	  golang:1.12 \
	  go build github.com/kube-ovs/kube-ovs/cmd/kube-ovs

.PHONY: build
build:
	docker build -t $(IMAGE_REPO):$(IMAGE_TAG) .
