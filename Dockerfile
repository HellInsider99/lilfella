FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/lilfella .

FROM python:3.13-alpine

RUN apk add --no-cache ca-certificates ffmpeg \
    && python -m venv /opt/yt-dlp \
    && /opt/yt-dlp/bin/pip install --no-cache-dir yt-dlp \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /app/logs \
    && chown -R app:app /app

ENV PATH="/opt/yt-dlp/bin:${PATH}"

COPY --from=build /out/lilfella /usr/local/bin/lilfella

WORKDIR /app
USER app

ENTRYPOINT ["lilfella"]
