IMAGE=liriorg/ostree-upload
DOCKER?=podman

image:
	@$(DOCKER) build -t $(IMAGE) .

push:
	@$(DOCKER) push $(shell podman images $(IMAGE) --format '{{.ID}}') docker://docker.io/$(IMAGE)
