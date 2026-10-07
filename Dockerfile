# Assembly-only image: binaries are cross-compiled outside Docker
# (see .github/workflows/docker.yml, or `just build` locally) and the web UI
# is embedded in them at compile time. The Docker build just picks the
# binary matching the target platform.

FROM alpine:3.21
ARG TARGETARCH
# bash: actions run via `bash -c`. curl: handy for healthchecks and actions.
# docker-cli: lets actions talk to a mounted /var/run/docker.sock.
RUN apk add --no-cache bash curl docker-cli
COPY --chmod=0755 bin/runic-linux-${TARGETARCH} /usr/local/bin/runic
WORKDIR /data
VOLUME /data
EXPOSE 1337
HEALTHCHECK --interval=30s --timeout=5s CMD \
  wget -q -O /dev/null http://127.0.0.1:1337/api/system || exit 1
ENTRYPOINT ["runic"]
CMD ["serve", "--config", "/data/config.yml"]
