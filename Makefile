IMAGE_NAME = ghcr.io/infosecstreams-mirror/streamstatus
IMAGE_TAG = $(shell git describe --tags --always --dirty --long)

.PHONY: help
help:
	@echo "make test"
	@echo "make build"
	@echo "make run"
	@echo "make push"
	@echo "make all"


.PHONE: test
test:
	cd src && go test -v ./...

.PHONY: build
build: test
	docker build --build-arg VERSION=$(IMAGE_TAG) -t $(IMAGE_NAME):$(IMAGE_TAG) .

.PHONY: run
run: build
	docker run -it --rm -p 8080:8080 \
	-e DATABASE_URL=postgres://username:password@localhost:5432/streamstatus?sslmode=disable \
	-e TWITCH_CLIENT_ID=client_id \
	-e TWITCH_CLIENT_SECRET=client_secret \
	-e SS_SECRETKEY=secret \
	-e SS_CALLBACK_URL=https://your-domain.com/api/webhook \
	$(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: push
push: build
	@echo "Are you sure you want to push? [y/N] " && read ans && [ $${ans:-N} = y ]
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(IMAGE_NAME):latest
	docker push $(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(IMAGE_NAME):latest

.PHONY: all
all: push

clean:
	docker rmi $(IMAGE_NAME):$(IMAGE_TAG)
