# Build with:
FROM golang:1.19-buster as builder
WORKDIR /app

ARG FOLDER
ARG VERSION

RUN apt-get update -y && apt-get install -y git

# Perform this build in two stages. First copy go.{mod,sum} and run
# go mod download to fetch dependencies. Separating these phases
# allows this step to be cached as a docker layer.
COPY ${FOLDER}/go.* ./
RUN go mod download

# Build the server.
COPY ${FOLDER} ./
RUN go build \
  -ldflags "-X github.com/thompsonja/discord_bots_common/pkg/version.Version=${VERSION}" \
  -v -o server

# Use a multi stage build using the debian slim image.
FROM debian:buster-slim
RUN apt-get update -y \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates \
  && rm -rf /var/lib/apt/lists/*

# Copy the server binary to the slim image
COPY --from=builder /app/server /server

CMD ["/server"]
