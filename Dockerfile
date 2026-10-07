# Assembly-only image: binaries are cross-compiled outside Docker
# (see .github/workflows/release.yml, or `just build` locally) and the web UI
# is embedded in them at compile time. The Docker build just picks the
# binary matching the target platform.
#
# Based on oven/bun:1-alpine (instead of plain alpine) so actions can run
# JavaScript/TypeScript directly with built-in bun support.

FROM oven/bun:1-alpine
ARG TARGETARCH
# bash: actions run via `bash -c`. curl: handy for actions. docker-cli: lets
# actions talk to a mounted /var/run/docker.sock. openssl: for actions
# needing TLS tooling.
RUN apk add --no-cache bash curl docker-cli openssl
COPY --chmod=0755 bin/runic-linux-${TARGETARCH} /usr/local/bin/runic
WORKDIR /data
VOLUME /data
EXPOSE 1337
HEALTHCHECK --interval=30s --timeout=5s CMD \
  wget -q -O /dev/null http://127.0.0.1:1337/api/system || exit 1
ENTRYPOINT ["runic"]
CMD ["serve", "--config", "/data/config.yml"]
