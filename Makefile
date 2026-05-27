IMAGE_NAME  ?= ghcr.io/scale-out/cert-manager-webhook-autodns
IMAGE_TAG   ?= 0.2.0

ENVTEST_K8S_VERSION ?= 1.31.0
OUT := $(shell pwd)/_out

$(shell mkdir -p "$(OUT)")

.PHONY: tidy build vet lint test image push template clean

tidy:
	go mod tidy

vet:
	go vet ./...

build:
	CGO_ENABLED=0 go build -o $(OUT)/webhook -ldflags '-w -extldflags "-static"' .

test:
	@which setup-envtest >/dev/null 2>&1 || go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	KUBEBUILDER_ASSETS="$$(setup-envtest use $(ENVTEST_K8S_VERSION) -p path)" go test -v ./...

image:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

push: image
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

template:
	helm template cert-manager-webhook-autodns deploy/cert-manager-webhook-autodns \
		--set groupName=acme.example.com \
		--set image.repository=$(IMAGE_NAME) \
		--set image.tag=$(IMAGE_TAG) > $(OUT)/rendered-manifest.yaml

clean:
	rm -Rf $(OUT)
