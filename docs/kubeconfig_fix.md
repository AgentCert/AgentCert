# Kubeconfig Fix for AgentCert Stack on WSL + Minikube (Docker driver)

## Problem

When the AgentCert GraphQL server starts on WSL and tries to talk to a local
minikube cluster, it fails with:

```
failed RBAC preflight: unable to create kubernetes client
```

or, when running `kubectl` against the converted kubeconfig:

```
error: host must be a URL or a host:port pair: "http/mnt/s/127.0.0.1:50888"
```

Two root causes:

1. The kubeconfig produced by Windows minikube uses **Windows-style paths**
   (`C:\Users\<user>\.minikube\...`) and `\` separators that the Linux
   `client-go` library cannot resolve from inside WSL.
2. The minikube API server port (`docker port minikube 8443`) **changes every
   time the minikube container restarts**, so any baked-in port becomes stale.
3. The original conversion `sed` rule was too greedy and rewrote the URL scheme
   `https://` into `http/mnt/s/` because it matched any `<letter>:/+` pattern.

## Environment

- WSL2 Ubuntu, user `devik`
- Windows kubeconfig at `C:\Users\devik\.kube\config` (mirrored to
  `/home/devik/.kube/config`)
- Minikube running with the `docker` driver, container name `minikube`
- AgentCert GraphQL binary expects `KUBECONFIG` / `KUBE_CONFIG_FILE_PATH` env
  vars; otherwise it falls back to `rest.InClusterConfig()` and fails

## Fix

### 1. Set kubeconfig env vars in `.env`

Add (or confirm) these two lines in [.env](../.env) so the GraphQL process
picks the converted kubeconfig at startup:

```env
KUBECONFIG=/home/devik/.kube/config
KUBE_CONFIG_FILE_PATH=/home/devik/.kube/config
```

The startup script
[scripts/azure_build/start-agentcert-v2.sh](../scripts/azure_build/start-agentcert-v2.sh)
copies `$KUBECONFIG` into a runtime-only file
`${PID_DIR}/kubeconfig.wsl` and rewrites paths/ports there. The original file
under `~/.kube/config` is never modified.

> `PID_DIR` defaults to `/tmp/agentcert-runtime` (set near the top of the
> startup script). All paths in this doc assume that default. If you override
> `PID_DIR`, substitute it everywhere you see `/tmp/agentcert-runtime/`.

**Pre-requisite:** the source kubeconfig referenced by `$KUBECONFIG` must
exist in WSL before the script runs. If `~/.kube/config` is missing, the copy
step silently produces nothing usable. Mirror it once with:

```bash
mkdir -p ~/.kube && cp /mnt/c/Users/$USER/.kube/config ~/.kube/config
```

### 2. Conversion logic (Windows -> WSL paths)

In [start-agentcert-v2.sh](../scripts/azure_build/start-agentcert-v2.sh)
(see lines 478-515 for the live block):

```bash
# Step 1: collapse all backslashes to forward slashes
sed -i 's#\\\\#/#g; s#\\#/#g' "${KUBECONFIG_WSL}"

# Step 2: rewrite C:/Users/<winuser> -> /mnt/c/Users/<winuser>
win_user="$(grep -oE '[A-Za-z]:[\\/]+Users[\\/]+[^\\/[:space:]]+' \
            "${KUBECONFIG_WSL}" | head -1 | \
            sed -E 's#^[A-Za-z]:[\\/]+Users[\\/]+##')"
[ -z "$win_user" ] && win_user="$(id -un)"
sed -i -E "s#[Cc]:/+Users/+${win_user}#/mnt/c/Users/${win_user}#g" \
    "${KUBECONFIG_WSL}"

# Step 3 (PATCHED): handle other drive letters ONLY at YAML value position
#                   (after ': '). Without the prefix this rewrites 'https://'
#                   into 'http/mnt/s/'.
sed -i -E 's#(:[[:space:]]+)([A-Za-z]):/+#\1/mnt/\L\2/#g' "${KUBECONFIG_WSL}"
```

Why the `(:[[:space:]]+)` prefix matters:
- A YAML value line looks like `    certificate-authority: D:/foo/bar` -- the
  drive letter is preceded by `: `.
- A URL value looks like `    server: https://127.0.0.1:64150` -- here `s` is
  preceded by `http`, not `: `.
- Anchoring on `: <space>` ensures only YAML values get the drive-letter
  rewrite; URL schemes are left alone.

**Caveat on `win_user` detection:** Step 2 derives the Windows username from
the first `<drive>:/Users/<name>` path it finds in the kubeconfig. If that
lookup fails it falls back to `id -un` (the **Linux** username). On this
workstation the two happen to be the same (`devik`), so the fallback works
silently. On any machine where the Windows and WSL usernames differ, the
fallback will produce a broken `/mnt/c/Users/<linux-user>/...` path. If you
ever see `no such file or directory` for the CA / client-cert files, check
that the rewritten path matches the real Windows username.

### 3. Refresh minikube API server port

The minikube container picks a new random host port for `8443` after every
restart. Detect it and patch the `server:` line:

```bash
if docker ps --format '{{.Names}}' | grep -qx 'minikube'; then
  mk_api_port="$(docker port minikube 8443 | head -1 | awk -F: '{print $2}')"
  if [ -n "$mk_api_port" ]; then
    sed -i -E "0,/server: https:\/\/127\.0\.0\.1:[0-9]+/s##server: https://127.0.0.1:${mk_api_port}#" \
        "${KUBECONFIG_WSL}"
  fi
fi
```

> **This refresh runs only once, at stack startup.** If you `minikube stop`
> / `minikube start` (or restart Docker Desktop) while the AgentCert stack is
> still running, minikube will pick a new random host port for `8443` and the
> `server:` URL inside `kubeconfig.wsl` becomes stale. Symptom: `kubectl` and
> the GraphQL server start failing with
> `dial tcp 127.0.0.1:<old port>: connect: connection refused`. Either
> restart the AgentCert stack, or run the manual repair below to re-patch
> the port without a full restart.

### 4. Verify

```bash
KC=/tmp/agentcert-runtime/kubeconfig.wsl
head -20 "$KC"           # server: https://127.0.0.1:<live_port>
                         # certificate-authority: /mnt/c/Users/devik/.minikube/ca.crt
KUBECONFIG="$KC" kubectl get ns --request-timeout=15s
```

Expected: namespace list returns within seconds (no TLS or path errors).

## Manual repair (if you need to fix without restarting the stack)

If `/tmp/agentcert-runtime/kubeconfig.wsl` is already corrupted from an old
run, repair it in place. The file is owned by `root` (the startup runs under
`sudo`), so chown it first:

```bash
sudo chown $(id -un):$(id -gn) /tmp/agentcert-runtime/kubeconfig.wsl

# Replace the server URL with the live minikube port
PORT=$(docker port minikube 8443 | head -1 | awk -F: '{print $2}')
sed -i -E "s#^([[:space:]]*server:[[:space:]]+).*#\1https://127.0.0.1:${PORT}#" \
    /tmp/agentcert-runtime/kubeconfig.wsl

# If the URL was mangled to 'http/mnt/s/...', the same sed above fixes it.

# Confirm
grep -E 'server:|certificate-authority:' /tmp/agentcert-runtime/kubeconfig.wsl
KUBECONFIG=/tmp/agentcert-runtime/kubeconfig.wsl kubectl get ns
```

## Common pitfalls

| Symptom | Cause | Action |
|--------|-------|--------|
| `host must be a URL or a host:port pair: "http/mnt/s/..."` | Step 3 regex too greedy | Use the patched regex with `(:[[:space:]]+)` prefix |
| `dial tcp 127.0.0.1:<old port>: connect: connection refused` | Stale minikube port | Re-run port refresh or restart the stack |
| `unable to load in-cluster configuration` | `KUBECONFIG` env var not exported to the GraphQL process | Confirm both `KUBECONFIG` and `KUBE_CONFIG_FILE_PATH` are in [.env](../.env) |
| `open /home/devik/.kube/config: no such file or directory` | WSL home doesn't have the kubeconfig | `cp /mnt/c/Users/devik/.kube/config /home/devik/.kube/config` |
| `Permission denied` writing `/tmp/agentcert-runtime/kubeconfig.wsl` | File owned by root from a previous sudo run | `sudo chown $(id -un):$(id -gn) /tmp/agentcert-runtime/kubeconfig.wsl` |

## References

- Conversion logic: [start-agentcert-v2.sh#L478-L515](../scripts/azure_build/start-agentcert-v2.sh#L478-L515)
- Env file: [.env](../.env)
