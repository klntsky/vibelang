# The converter's compile worker: credential-free, one CPU, 512 MiB, no egress. Build for linux/amd64:
#   docker build --platform linux/amd64 --build-context tarballs=dist-npm/tarballs -t effeck-converter-worker packages/converter-worker
# Run (see README.md): docker run --cpus 1 --memory 512m --pids-limit 256 --read-only --tmpfs /tmp \
#   --cap-drop ALL --security-opt no-new-privileges -p 8080:8080 effeck-converter-worker
FROM node:26-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends util-linux && rm -rf /var/lib/apt/lists/*
WORKDIR /opt/converter
COPY package.json compile.mjs server.mjs ./
COPY project/package.json project/tsconfig.json project/
# effeck and effeck-runtime come from the release tarballs until 0.0.1 is on npm; effect from the registry.
COPY --from=tarballs effeck-0.0.1.tgz effeck-runtime-0.0.1.tgz effeck-linux-x64-0.0.1.tgz /tmp/tarballs/
RUN cd project && npm install --no-audit --no-fund --omit=dev /tmp/tarballs/*.tgz effect@4.0.5 && rm -rf /tmp/tarballs /root/.npm
USER node
# A pod cannot create network namespaces, so the deployment denies egress (deploy/network-policy.yaml)
# and the server refuses to start if it can reach the internet anyway.
ENV EFFECK_WORKER_NETWORK=policy PORT=8080 NODE_ENV=production
EXPOSE 8080
CMD ["node", "server.mjs"]
