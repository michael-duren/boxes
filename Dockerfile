# build first
FROM golang:latest AS builder
WORKDIR /src

# handle deps first so it's cached
COPY go.mod go.sum ./
RUN go mod download

# actually source and build
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/box ./cmd/cli

# run interactive container
FROM debian:bookworm-slim
WORKDIR /
COPY --from=builder /bin/box /bin/box
COPY ./alpinefs/ alpinefs/
COPY Makefile.container Makefile
RUN apt-get update && apt-get install -y make \
  && rm -rf /var/lib/apt/lists/*

# convience aliases
RUN echo 'alias l="ls -la"' >> /root/.bashrc

CMD ["/bin/bash"]
