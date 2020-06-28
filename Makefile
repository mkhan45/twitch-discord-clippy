fmt:
	go fmt .

build: fmt
	go build .

run: export TWITCH_CLIENT_ID = $(shell cat secrets/client_id)
run: export TWITCH_CLIENT_SECRET = $(shell cat secrets/client_secret)
run: export DISCORD_TOKEN = $(shell cat secrets/discord_token)
run: export TWITCH_REDIRECT_URI = $(shell cat secrets/redirect_uri)
run: fmt
	go run .
	
daemon: export TWITCH_CLIENT_ID = $(shell cat secrets/client_id)
daemon: export TWITCH_CLIENT_SECRET = $(shell cat secrets/client_secret)
daemon: export DISCORD_TOKEN = $(shell cat secrets/discord_token)
daemon: export TWITCH_REDIRECT_URI = $(shell cat secrets/redirect_uri)
daemon: fmt
		rm log.txt
		nohup go run . > log.txt
