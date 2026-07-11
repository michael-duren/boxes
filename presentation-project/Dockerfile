# syntax=docker/dockerfile:1
#
# "evil node" image for the container-isolation talk.
#
# The real node interpreter is preserved as `node.real`; the harmless,
# read-only escape probe (scripts/evilnode.sh) takes its place as `node`, and
# the sample app (webapp/) is baked in. `docker export` a container from this
# image to get a flat rootfs (see scripts/setup.sh).
#
ARG BASE=node:latest
FROM ${BASE}

# --- backdoor: keep the genuine interpreter, install the probe as `node` ---
RUN mv /usr/local/bin/node /usr/local/bin/node.real
COPY scripts/evilnode.sh /usr/local/bin/node
RUN chmod +x /usr/local/bin/node

# --- sample application (zero-dependency http server) ---
WORKDIR /app
COPY webapp/ /app/

ENV EVILNODE_LOG=/var/log/evilnode.log \
    PORT=3000
EXPOSE 3000

# The container serves the sample app. Because `node` is the probe, this runs
# the escape probe first (writing the log), then hands off to the real
# interpreter to start the server -- which then displays that very log.
CMD ["node", "/app/server.js"]
