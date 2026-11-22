# Lifefella

A Telegram bot that matches tracks across Spotify, Deezer, YouTube Music, and Apple Music. It can also download media from supported X/Twitter, Instagram, Reddit, YouTube Shorts, and TikTok links. The Docker image includes yt-dlp and ffmpeg.

## Run with Docker Compose

1. Copy the example environment file and add your Telegram bot token:

   ```sh
   Copy-Item .env.example .env
   ```

   `TELEGRAM_BOT_TOKEN` is required. Spotify matching uses `SPOTIFY_CLIENT_ID` and `SPOTIFY_CLIENT_SECRET`, YouTube Music matching uses `YOUTUBE_API_KEY`. The Deezer and Apple Music integrations do not require API keys.

2. Build the image and start the bot:

   ```sh
   docker compose up --build -d
   ```

3. Follow the bot logs:

   ```sh
   docker compose logs -f bot
   ```

## Useful commands

```sh
docker compose ps                  # Show container status
docker compose restart bot         # Restart the bot
docker compose up --build -d       # Rebuild and restart after changes
docker compose down                # Stop and remove the container
```

Logs are stored in the `bot-logs` Docker volume and remain available after `docker compose down`. To remove the container and its persisted logs, run:

```sh
docker compose down --volumes
```
