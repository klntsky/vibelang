# Build from the repository root with deploy/cloudbuild.yaml. No local release artifacts.
FROM golang:1.27-bookworm AS compiler
WORKDIR /src
COPY typescript/tsc/go.mod typescript/tsc/go.sum ./
RUN go mod download
COPY typescript/tsc/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /vibelang ./cmd/tsc

FROM node:26-bookworm-slim AS packages
WORKDIR /src
COPY LICENSE ./
COPY typescript/LICENSE.txt typescript/
COPY packages/runtime/ packages/runtime/
RUN cd packages/runtime && npm install --ignore-scripts --no-audit --no-fund && npm run build
COPY packages/compiler/ packages/compiler/
COPY packages/converter-worker/prepare-packages.mjs packages/converter-worker/
COPY --from=compiler /vibelang packages/compiler/native/vibelang
RUN node packages/converter-worker/prepare-packages.mjs /tarballs

FROM node:26-bookworm-slim AS installed
WORKDIR /opt/converter
COPY packages/converter-worker/project/ project/
COPY --from=packages /tarballs/ /tmp/tarballs/
RUN cd project && npm install --ignore-scripts --no-audit --no-fund --omit=dev /tmp/tarballs/*.tgz \
    && rm -rf /tmp/tarballs /root/.npm

FROM node:26-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends util-linux \
    && rm -rf /var/lib/apt/lists/* /usr/local/lib/node_modules /opt/yarn* \
    && rm -f /bin/sh /bin/bash /bin/dash /usr/bin/sh /usr/bin/bash /usr/bin/dash
WORKDIR /opt/converter
COPY --from=installed /opt/converter/project/ project/
COPY packages/converter-worker/package.json packages/converter-worker/compile.mjs packages/converter-worker/server.mjs ./
USER 1000:1000
ENV VIBELANG_WORKER_NETWORK=policy PORT=8080 NODE_ENV=production TMPDIR=/tmp
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/node"]
CMD ["server.mjs"]
