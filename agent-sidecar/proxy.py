"""
agent-sidecar – Transparent metadata-injection HTTP proxy.
==========================================================

Sits between any LLM agent and LiteLLM to enrich requests with
agent identity. The agent has ZERO awareness of experiment IDs.

Env vars read at startup:
  SIDECAR_PORT        – listen port (default 4001)
  UPSTREAM_URL        – real LiteLLM base URL (e.g. http://litellm:4000)
  INJECTION_MODE      – how to inject context (default "openai-metadata")
                        "openai-metadata" : merge into JSON body metadata dict
                        "http-header"     : add X-Experiment-* request headers
                        "none"            : pure passthrough, no injection
  AGENT_ID            – injected via Helm --set agentId=<uuid> at deploy time

  ALLOWED_METADATA_KEYS – comma-separated allowlist of env-var names to inject.
                          Default: "AGENT_ID" (routing/billing only).
                          To restore legacy behaviour (NOT recommended):
                          "AGENT_ID,EXPERIMENT_ID,EXPERIMENT_RUN_ID,WORKFLOW_NAME"

Security note:
  Experiment context (EXPERIMENT_ID, EXPERIMENT_RUN_ID, WORKFLOW_NAME) is
  intentionally excluded by default to enforce the "blind observer" principle.
  The flash-agent must detect anomalies from system signals, not from
  prior knowledge of what faults are being injected.
  See: docs/Flash-agent-data-leakage-analysis.md
"""

import json
import os
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.request import Request, urlopen
from urllib.error import HTTPError, URLError

SIDECAR_PORT = int(os.environ.get("SIDECAR_PORT", "4001"))
UPSTREAM_URL = os.environ.get("UPSTREAM_URL", "http://localhost:4000").rstrip("/")
INJECTION_MODE = os.environ.get("INJECTION_MODE", "openai-metadata").lower()

# Metadata allowlist – controls which env vars are injected into LLM requests.
# Default: only AGENT_ID (routing/billing). Experiment context is excluded to
# prevent the flash-agent from receiving prior knowledge of fault injection.
_ALLOWED_KEYS_RAW = os.environ.get("ALLOWED_METADATA_KEYS", "AGENT_ID")
ALLOWED_METADATA_KEYS = frozenset(
    k.strip().upper() for k in _ALLOWED_KEYS_RAW.split(",") if k.strip()
)

# Metadata context – read once at startup, immutable for pod lifetime.
# Only keys present in ALLOWED_METADATA_KEYS are included.
_ALL_KNOWN_KEYS = ("AGENT_ID", "EXPERIMENT_ID", "EXPERIMENT_RUN_ID", "WORKFLOW_NAME")
EXPERIMENT_CONTEXT = {}
for _key in _ALL_KNOWN_KEYS:
    if _key in ALLOWED_METADATA_KEYS:
        _val = os.environ.get(_key, "")
        if _val:
            EXPERIMENT_CONTEXT[_key.lower()] = _val

# Headers to strip (hop-by-hop)
_HOP_HEADERS = frozenset(("host", "transfer-encoding"))


class ProxyHandler(BaseHTTPRequestHandler):
    """Forward requests to upstream LiteLLM, injecting metadata on POST."""

    def do_POST(self):
        body = self._read_body()
        extra_headers = {}
        if body and EXPERIMENT_CONTEXT:
            if INJECTION_MODE == "openai-metadata":
                body = self._inject_metadata(body)
            elif INJECTION_MODE == "http-header":
                extra_headers = self._build_context_headers()
            # "none" or unknown → pure passthrough
        self._proxy(body, extra_headers=extra_headers)

    def do_GET(self):
        self._proxy(None)

    def do_PUT(self):
        self._proxy(self._read_body())

    def do_DELETE(self):
        self._proxy(None)

    def do_OPTIONS(self):
        self._proxy(None)

    # ── helpers ──────────────────────────────────────────────────────

    def _read_body(self):
        length = int(self.headers.get("Content-Length", 0))
        return self.rfile.read(length) if length > 0 else b""

    @staticmethod
    def _inject_metadata(body: bytes) -> bytes:
        """Merge experiment context into the top-level 'metadata' dict.

        The OpenAI Python SDK sends ``extra_body={"metadata": {...}}``
        which becomes a top-level ``metadata`` key in the HTTP JSON body.
        LiteLLM reads this and forwards it to Langfuse.
        """
        try:
            data = json.loads(body)
            if isinstance(data, dict):
                metadata = data.setdefault("metadata", {})
                metadata.update(EXPERIMENT_CONTEXT)
                return json.dumps(data).encode("utf-8")
        except (json.JSONDecodeError, ValueError):
            pass  # non-JSON body – forward as-is
        return body

    @staticmethod
    def _build_context_headers() -> dict:
        """Return experiment context as X-Experiment-* HTTP headers."""
        return {
            f"X-Experiment-{k.replace('_', '-').title()}": v
            for k, v in EXPERIMENT_CONTEXT.items()
        }

    def _proxy(self, body, *, extra_headers=None):
        upstream = f"{UPSTREAM_URL}{self.path}"

        # Forward headers, skipping hop-by-hop
        headers = {
            k: v
            for k, v in self.headers.items()
            if k.lower() not in _HOP_HEADERS
        }
        if extra_headers:
            headers.update(extra_headers)
        if body is not None:
            headers["Content-Length"] = str(len(body))

        try:
            req = Request(upstream, data=body, headers=headers, method=self.command)
            with urlopen(req, timeout=300) as resp:
                resp_body = resp.read()
                self.send_response(resp.status)
                for key, val in resp.getheaders():
                    if key.lower() not in ("transfer-encoding",):
                        self.send_header(key, val)
                self.end_headers()
                self.wfile.write(resp_body)
        except HTTPError as e:
            resp_body = e.read()
            self.send_response(e.code)
            for key, val in e.headers.items():
                if key.lower() not in ("transfer-encoding",):
                    self.send_header(key, val)
            self.end_headers()
            self.wfile.write(resp_body)
        except URLError as e:
            self.send_error(502, f"Upstream unreachable: {e.reason}")

    def log_message(self, fmt, *args):
        print(f"[agent-sidecar] {self.client_address[0]} {args[0]}", flush=True)


def main():
    print(f"[agent-sidecar] Starting on :{SIDECAR_PORT} → {UPSTREAM_URL}  mode={INJECTION_MODE}", flush=True)
    print(f"[agent-sidecar] Allowed metadata keys: {sorted(ALLOWED_METADATA_KEYS)}", flush=True)
    if EXPERIMENT_CONTEXT:
        print(f"[agent-sidecar] Injecting: {list(EXPERIMENT_CONTEXT.keys())}", flush=True)
    else:
        print("[agent-sidecar] No metadata to inject — transparent passthrough", flush=True)
    _filtered = [k for k in _ALL_KNOWN_KEYS if k not in ALLOWED_METADATA_KEYS and os.environ.get(k)]
    if _filtered:
        print(f"[agent-sidecar] Filtered out (present but not allowed): {_filtered}", flush=True)

    server = HTTPServer(("0.0.0.0", SIDECAR_PORT), ProxyHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    server.server_close()


if __name__ == "__main__":
    main()
