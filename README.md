# Lifefella

A Telegram bot that matches tracks across Spotify, Deezer, YouTube Music, and Apple Music. It can also download media from supported X/Twitter, Instagram, Reddit, YouTube Shorts, and TikTok links. The Docker image includes yt-dlp and ffmpeg.

## Run with Docker Compose

1. Copy the example environment file and add your Telegram bot token:

   ```sh
   Copy-Item .env.example .env
   ```

   `TELEGRAM_BOT_TOKEN` is required. Spotify matching uses `SPOTIFY_CLIENT_ID` and `SPOTIFY_CLIENT_SECRET`, YouTube Music matching uses `YOUTUBE_API_KEY`. The Deezer and Apple Music integrations do not require API keys.

   To download Reddit media, place your Reddit `reddit-cookies.txt` file next to `docker-compose.yml` on the server. Keep this file private and do not commit it.

2. Build the image and start the bot:

   ```sh
   sudo docker compose up --build -d
   ```

   If you add or replace the cookies file while the bot is running, recreate the container to apply the mount:

   ```sh
   sudo docker compose up -d --force-recreate bot
   ```

3. View the bot logs:

   ```sh
   sudo docker compose logs -f bot
   ```

   This follows new log messages in real time. Press `Ctrl+C` to stop viewing logs; the bot keeps running. To show only the most recent 100 lines, use:

   ```sh
   sudo docker compose logs --tail=100 bot
   ```

## Useful commands

```sh
sudo docker compose ps                  # Show container status
sudo docker compose restart bot         # Restart the bot
sudo docker compose up --build -d       # Rebuild and restart after changes
sudo docker compose down                # Stop and remove the container
```

Logs are stored in the `bot-logs` Docker volume and remain available after `docker compose down`. Avoid `--volumes` if you want to keep them. To remove the container and its persisted logs, run:

```sh
sudo docker compose down --volumes
```
