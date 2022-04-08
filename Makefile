SHELL := /bin/bash
IMAGE := quay.io/utilitywarehouse/kube-applier

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: manifests generate controller-gen test build run release

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) \
		crd \
		paths="./..." \
		output:crd:artifacts:config=manifests/base/cluster
	@{ \
	cd manifests/base/cluster ;\
	kustomize edit add resource kube-applier.io_* ;\
	}

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Make sure controller-gen package is installed in a module-aware mode, ignoring
# the local go.mod.
controller-gen:
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.8.0

KUBEBUILDER_BINDIR=$${PWD}/kubebuilder-bindir
KUBEBUILDER_VERSION="1.23.x"
test:
	command -v setup-envtest || go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	mkdir -p $(KUBEBUILDER_BINDIR)
	setup-envtest --bin-dir $(KUBEBUILDER_BINDIR) use -p env $(KUBEBUILDER_VERSION)
	source <(setup-envtest --bin-dir $(KUBEBUILDER_BINDIR) use -i -p env $(KUBEBUILDER_VERSION)); CGO_ENABLED=1; go test -v -race -count=1 -cover ./...

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
