BINARY_NAME=packet-capture
IMAGE_NAME=packet-capture:latest
KIND_CLUSTER=kind
NAMESPACE=kube-system

.PHONY: all build docker build-docker kind deploy undeploy clean

all: build docker-build kind-load deploy

help: ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the Go binary locally
	go build -o $(BINARY_NAME) .

docker-build: ## Build the Docker image
	docker build -t $(IMAGE_NAME) .

kind-load: ## Load the Docker image into the kind cluster
	kind load docker-image $(IMAGE_NAME) --name $(KIND_CLUSTER)

deploy: ## Deploy the RBAC and DaemonSet manifests to the cluster
	kubectl apply -f ./manifests/rbac.yaml
	kubectl apply -f ./manifests/daemonset.yaml

undeploy: ## Undeploy the DaemonSet and RBAC manifests from the cluster
	kubectl delete -f ./manifests/daemonset.yaml --ignore-not-found
	kubectl delete -f ./manifests/rbac.yaml --ignore-not-found

test-pod: ## Deploy the traffic generator test pod
	kubectl apply -f ./manifests/test-pod.yaml -n $(NAMESPACE)

clean: ## Remove binary, local iamges and delete test pod 
	rm -rf bin/
	kubectl delete -f ./manifests/test-pod.yaml --ignore-not-found
