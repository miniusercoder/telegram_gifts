IMAGE_NAME = tg-lib-builder
CONTAINER_NAME = tg-lib-build
ARTIFACTS_DIR = ./dist

build-libtg:
	DOCKER_BUILDKIT=1 docker build --progress=plain -t $(IMAGE_NAME) .
	mkdir -p $(ARTIFACTS_DIR)
	docker create --name $(CONTAINER_NAME) $(IMAGE_NAME)
	docker cp $(CONTAINER_NAME):/build/libtg.so $(ARTIFACTS_DIR)/
	docker cp $(CONTAINER_NAME):/build/libtg.h $(ARTIFACTS_DIR)/
	docker rm $(CONTAINER_NAME)
