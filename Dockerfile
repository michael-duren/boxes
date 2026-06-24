ARG BIN_PATH=/bin/box

# build first
FROM golang:latest AS builder
ARG BIN_PATH
WORKDIR /src

# handle deps first so it's cached
COPY go.mod go.sum ./
RUN go mod download

# actually source and build
COPY . .
# disable c linking and build
RUN CGO_ENABLED=0 go build -o ${BIN_PATH} ./cmd/cli

# run interactive container
FROM debian:bookworm-slim
ARG BIN_PATH
WORKDIR /
COPY --from=builder ${BIN_PATH} ${BIN_PATH}
COPY ./alpinefs/ alpinefs/
COPY Makefile.container Makefile
RUN apt-get update && apt-get install -y make \
  && rm -rf /var/lib/apt/lists/*

# convience aliases
RUN echo 'alias l="ls -la"' >> /root/.bashrc

CMD ["/bin/bash"]
