IMAGE_NAME = tg-lib-builder
CONTAINER_NAME = tg-lib-build
ARTIFACTS_DIR = ./dist

build-libtg:
	DOCKER_BUILDKIT=1 docker build --progress=plain -t $(IMAGE_NAME) .
	mkdir -p $(ARTIFACTS_DIR)
	docker create --name $(CONTAINER_NAME) $(IMAGE_NAME)
	docker cp $(CONTAINER_NAME):/dist/libtg.so $(ARTIFACTS_DIR)/ || true
	docker cp $(CONTAINER_NAME):/dist/libtg.h $(ARTIFACTS_DIR)/ || true
	docker rm $(CONTAINER_NAME)
