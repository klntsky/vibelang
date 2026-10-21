# Converter compile worker

The worker checks one VibeLang source file with the pinned compiler and dependencies:
it runs `vibelang --noEmit --pretty false -p <fixed project>` in a fresh temporary
directory and never runs the submitted program. `POST /compile` accepts
`{"source":"..."}` and returns the exit code, bounded diagnostics, compiler version,
compiler digest, and dependencies digest. `GET /healthz` returns the worker identity.

Each compile has one CPU, a 512 MiB container memory limit, and a 10-second wall
and CPU limit; compiles run serially. The process runs as UID/GID 1000 without
credentials, capabilities, a service account token, or a writable root filesystem.
Only `/tmp` is writable: a 64 MiB memory-backed volume counted against the memory
limit. Source is limited to 8 KiB; diagnostic output to 64 KiB, 50 records, and 500
characters per message. Logs omit source, diagnostic text, and client addresses.

## Build and test

Cloud Build builds the compiler and runtime from this repository, packs their
existing packages, and installs the fixed project with `effect@4.0.5`; no package
is published. The final image has no shell or npm. Cloud Build uses the existing
narrow `plue-image-builder` account and records verified provenance. The smoke
step runs the final image with `--network=none`, one CPU, 512 MiB, a read-only
root, no capabilities, and a bounded temporary filesystem, and checks the UID,
absent shells, health endpoint, hero compile, and type error.

From the repository root, submit a clean source revision with a unique tag:

```sh
export LANE=ek-worker-deploy
revision=$(jj log -r @ --no-graph -T commit_id)
gcloud builds submit . --project plue-prod-1771780303 --region us-central1 \
  --config packages/converter-worker/deploy/cloudbuild.yaml \
  --ignore-file packages/converter-worker/deploy/build.gcloudignore \
  --gcs-source-staging-dir gs://plue-prod-1771780303_cloudbuild/source \
  --substitutions "_REVISION=$revision"
```

The Artifact Registry repository has immutable tags. Record `results.images[].digest`
from `gcloud builds describe` and pin it in `deploy/deployment.yaml`.
`deploy/image.json` holds the staged build receipt.

For worker tests on the Linux build host, build the native compiler and runtime,
then run these commands through `scripts/remote.sh`:

```sh
node packages/converter-worker/prepare-packages.mjs dist-npm/tarballs
sh packages/converter-worker/test/setup-project.sh dist-npm/tarballs
VIBELANG_WORKER_NETWORK=policy node --test packages/converter-worker/test/*.test.mjs
```

Policy-mode host tests don't prove Kubernetes network isolation. The Cloud Build
image smoke test proves the process works in an isolated container; the hosting
DRI must verify the cluster policies after applying them.

## Operator runbook

### Placement and prerequisites

Use project `plue-prod-1771780303`, cluster `plue-sandbox-cluster`, region
`us-central1`, and Artifact Registry repository `smithers`. These match
`~/plue/infra/terraform/environments/prod` and `modules/registry`.

The backend runs in namespace `smithers`, with pod labels `app=smithers-api`,
`app.kubernetes.io/name=smithers`, `app.kubernetes.io/instance=smithers`, and
`app.kubernetes.io/component=api`. The worker uses the existing `default`
namespace: the Helm chart's `templates/cluster-dns-policy.yaml` grants DNS to
every pod in `smithers`, and NetworkPolicy allow rules are additive, so a worker
there would keep DNS egress even with `egress: []`. At staging time, `default`
had no NetworkPolicies or workloads. Add no policy selecting the worker that
permits egress, including DNS or metadata access.

`network-policy.yaml` permits incoming TCP/8080 only from those backend pods in
`smithers` and denies all worker egress. `backend-egress.yaml` is a new, narrow
policy letting the API reach the worker; it doesn't edit Helm-managed policies.
`namespace.yaml` references the existing namespace without changing its labels
or security configuration; don't delete that namespace on rollback.

Before binding its HTTP port, the server checks an outbound TCP connection to
`1.1.1.1:443`; a successful connection or a probe execution fault exits the
server. The HTTP startup probe can pass only after this check succeeds. It is a
misconfiguration check; NetworkPolicy enforces isolation for all destinations.
Keep policies in place throughout rollouts and rollback.

### Validate and apply

The hosting DRI applies the manifests during the approved rollout; staging uses
only server dry-run. From the repository root:

```sh
gcloud container clusters get-credentials plue-sandbox-cluster \
  --region us-central1 --project plue-prod-1771780303 --dns-endpoint
kubectl --context gke_plue-prod-1771780303_us-central1_plue-sandbox-cluster \
  apply --dry-run=server -k packages/converter-worker/deploy
```

Before applying, confirm the pinned image and review every policy selecting the
worker. Apply the network policies before creating the Deployment:

```sh
kubectl apply -f packages/converter-worker/deploy/namespace.yaml
kubectl apply -f packages/converter-worker/deploy/network-policy.yaml
kubectl apply -f packages/converter-worker/deploy/backend-egress.yaml
kubectl apply -f packages/converter-worker/deploy/service.yaml
kubectl apply -f packages/converter-worker/deploy/deployment.yaml
kubectl -n default rollout status deployment/effeck-converter-worker --timeout=120s
kubectl -n default logs deployment/effeck-converter-worker
```

If startup fails, inspect `kubectl -n default describe pod -l app=effeck-converter-worker`
and the previous container logs. An egress error requires fixing policy
selection or enforcement. Keep conversion disabled while investigating.

### Smoke-test

First check the compiled image through a local port forward, without creating a
test pod. Run the forward in another terminal:

```sh
kubectl -n default port-forward service/effeck-converter-worker 18080:8080
```

From the repository root, POST the held-out hero and a type error. Neither is
executed:

```sh
node --input-type=module <<'JS'
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
const post = async source => {
  const response = await fetch("http://127.0.0.1:18080/compile", {
    method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ source }), signal: AbortSignal.timeout(15_000)
  })
  assert.equal(response.status, 200)
  return response.json()
}
const hero = readFileSync(".smithers/flows/convert/golden/01-hero-batch-lookup/expected.vibe", "utf8")
const good = await post(hero)
assert.equal(good.exitCode, 0)
assert.equal(good.timedOut, false)
assert.deepEqual(good.diagnostics, [])
const bad = await post('export function f(): number!! { return "no" }')
assert.notEqual(bad.exitCode, 0)
assert.ok(bad.diagnostics.length > 0)
console.log({ heroExitCode: good.exitCode, typeErrorDiagnostics: bad.diagnostics.length,
  compilerDigest: good.compilerDigest, dependenciesDigest: good.dependenciesDigest })
JS
```

A port forward bypasses Service routing and policy checks, so also POST the same
fixtures from the Smithers API's compile activity, confirm worker egress is
blocked, and confirm a caller without the backend identity is denied. The image
has Node but no shell; test egress with a direct Node command:

```sh
kubectl -n default exec deployment/effeck-converter-worker -- /usr/local/bin/node \
  --input-type=module -e 'import {proveNoEgress,settings} from "./compile.mjs"; if (!proveNoEgress(settings())) process.exit(1)'
```

### Point Smithers at the Service

Configure the backend operator setting, never repository-supplied flow input:

```text
SMITHERS_GUEST_COMPILE_ENDPOINT=http://effeck-converter-worker.default.svc.cluster.local:8080/compile
```

This setting is `guest.compile_endpoint` in the `eh-convert` backend change
`dbbd5a50bfb6ab61d3b96742592eab1d0b936cc0`, whose handoff was awaiting landing
when this worker was staged. The activity POSTs directly to the complete URL, so
include `/compile`; it bounds responses and times out at 15 seconds. An empty
setting disables conversion execution. Apply the setting only after that backend
change lands and the worker passes its smoke checks. Receipt signing keys stay
in the backend; the worker receives none. This runbook doesn't enable guest
access, change budgets, or bypass the golden gate.

### Roll back

Clear `SMITHERS_GUEST_COMPILE_ENDPOINT` or restore its previously approved value
through the backend's normal operator configuration path. For an earlier worker
revision, restore its recorded digest in the Deployment manifest, dry-run, and
apply it, or use the Deployment's retained history:

```sh
kubectl -n default rollout history deployment/effeck-converter-worker
kubectl -n default rollout undo deployment/effeck-converter-worker --to-revision=REVISION
kubectl -n default rollout status deployment/effeck-converter-worker --timeout=120s
```

For the initial release, disable the backend setting and delete only the worker
Deployment and Service. To remove the integration completely, delete its two
named NetworkPolicies after its pods terminate. Preserve `default` and all
existing infrastructure. Never remove the deny policy while a worker pod runs.
