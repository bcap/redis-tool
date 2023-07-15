.DEFAULT_GOAL := build

build:
	docker build -t bcap/redis-tool:latest .

shell: build
	docker run --rm -it --entrypoint /bin/bash bcap/redis-tool:latest

build-multi-arch:
	docker buildx build --platform linux/arm64,linux/amd64 --tag bcap/redis-tools:latest .