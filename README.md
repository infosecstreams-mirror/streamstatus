# StreamStatus (infosecstreams-mirror)

This is the backend for the [infosecstreams-mirror](https://github.com/infosecstreams-mirror) project. It is a Go-based REST API and Twitch EventSub Webhook receiver that tracks the online/offline status of cybersecurity streamers.

Instead of statically generating markdown files, this new architecture stores streamer activity in a **PostgreSQL database** and serves it dynamically to the frontend via a REST API.

## How It Works

1. Users add a new streamer to the database (currently via API/CLI).
2. The backend fetches their Twitch User ID and registers a subscription with Twitch's EventSub system for `stream.online`, `stream.offline`, and `channel.update` events.
3. When Twitch sends a webhook notification, the backend verifies the cryptographic signature (using `SS_SECRETKEY`) and updates the streamer's `is_online`, `game`, `language`, `tags`, and `last_seen` metadata in the PostgreSQL database.
4. The frontend fetches the live leaderboard from `/api/streamers`.

## Environment Variables

To run the backend, you need to configure the following environment variables:

```shell
# Port to listen on (Default: 3000)
export SS_PORT=3000

# Secret key used to securely sign and verify Twitch EventSub payloads
export SS_SECRETKEY=your_secure_secret_key

# PostgreSQL Connection String
export DATABASE_URL="postgres://user:password@localhost:5432/streamstatus?sslmode=disable"

# The publicly accessible URL where Twitch will send webhook events
export SS_CALLBACK_URL="https://api.yourdomain.com/webhook/callbacks"

# Twitch API Credentials (requires a developer application at dev.twitch.tv)
export TW_CLIENT_ID=your_twitch_client_id
export TW_CLIENT_SECRET=your_twitch_client_secret
```

## Endpoints

- `GET /api/status` - Healthcheck endpoint
- `GET /api/streamers` - Returns all tracked streamers and their current status (optionally filter with `?status=online` or `?status=offline`)
- `POST /webhook/callbacks` - Internal endpoint used exclusively by Twitch EventSub

## Running with Docker

We provide a durable `alpine`-based Docker image.

```shell
docker build -t streamstatus:latest .

docker run -d \
  -p 3000:3000 \
  -e SS_PORT=3000 \
  -e SS_SECRETKEY=your_secret \
  -e DATABASE_URL="postgres://user:pass@db:5432/streamstatus?sslmode=disable" \
  -e SS_CALLBACK_URL="https://api.yourdomain.com/webhook/callbacks" \
  -e TW_CLIENT_ID=client_id \
  -e TW_CLIENT_SECRET=client_secret \
  streamstatus:latest
```

## Running Directly (Bare Metal)

```shell
go build -o StreamStatus ./src/...
./StreamStatus
```
