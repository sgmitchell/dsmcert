build:
	go build -o dsmcert .

fmt:
	go fmt

test: build
	go test ./...

docker-build:
	@docker build -t dsmcert --build-arg=GITHUB_TOKEN="${GITHUB_TOKEN}" .

docker-push-ssh: docker-build
	docker save dsmcert | bzip2 | ssh server docker load