fmt:
	go fmt .

build: fmt
	go build .

run: export TWITCH_CLIENT_ID = $(shell cat secrets/client_id)
run: export TWITCH_CLIENT_SECRET = $(shell cat secrets/client_secret)
run: fmt
	go run .
