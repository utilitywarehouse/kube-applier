SHELL := /bin/bash
IMAGE := quay.io/utilitywarehouse/kube-applier

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: manifests generate controller-gen-install test build run release

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen-install
	controller-gen \
		crd:crdVersions=v1 \
		paths="./..." \
		output:crd:artifacts:config=manifests/base/cluster
	@{ \
	cd manifests/base/cluster ;\
	kustomize edit add resource kube-applier.io_* ;\
	}

# Generate code
generate: controller-gen-install
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Make sure controller-gen is installed. This should build and install packages
# in module-aware mode, ignoring any local go.mod file
controller-gen-install:
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.11.3

KUBEBUILDER_BINDIR=$${PWD}/kubebuilder-bindir
KUBEBUILDER_VERSION="1.30.x"
test:
	command -v setup-envtest || go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	mkdir -p $(KUBEBUILDER_BINDIR)
	ASSETS=$$(setup-envtest --bin-dir $(KUBEBUILDER_BINDIR) use -p path $(KUBEBUILDER_VERSION)); \
	KUBEBUILDER_ASSETS="$$ASSETS" CGO_ENABLED=1 go test -v -race -count=1 -cover ./...

build:
	docker build -t kube-applier .

BJS_VERSION="5.1.0"
update-bootstrap-js:
	(cd /tmp/ && curl -L -O https://github.com/twbs/bootstrap/releases/download/v$(BJS_VERSION)/bootstrap-$(BJS_VERSION)-dist.zip)
	(cd /tmp/ && unzip bootstrap-$(BJS_VERSION)-dist.zip)
	cp /tmp/bootstrap-$(BJS_VERSION)-dist/js/bootstrap.js static/bootstrap/js/bootstrap.js

update-jquery-js:
	curl -o static/bootstrap/js/jquery.min.js https://code.jquery.com/jquery-3.6.0.min.js

release:
	@sd "$(IMAGE):master" "$(IMAGE):$(VERSION)" $$(rg -l -- $(IMAGE) manifests/)
	@git add -- manifests/
	@git commit -m "Release $(VERSION)"
	@sd "$(IMAGE):$(VERSION)" "$(IMAGE):master" $$(rg -l -- "$(IMAGE)" manifests/)
	@git add -- manifests/
	@git commit -m "Clean up release $(VERSION)"
