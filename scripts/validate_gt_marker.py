import base64
import importlib.util
import json
import pathlib
import tempfile


def main() -> int:
    proxy_path = pathlib.Path("/mnt/d/Studies/AgentCert/agent-sidecar/proxy.py")
    spec = importlib.util.spec_from_file_location("sidecar_proxy", proxy_path)
    if spec is None or spec.loader is None:
        print("FAIL: unable to load sidecar module")
        return 2

    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)

    with tempfile.TemporaryDirectory() as temp_dir:
        gt_payload = {"disk-fill": {"expected": "x"}}
        gt_b64 = base64.b64encode(json.dumps(gt_payload).encode("utf-8")).decode("utf-8")
        pathlib.Path(temp_dir, "GROUND_TRUTH_JSON").write_text(gt_b64, encoding="utf-8")
        module.CONFIG_MOUNT = temp_dir

        context = {
            "notify_id": "nid-1",
            "experiment_id": "exp-1",
            "experiment_run_id": "run-1",
            "workflow_name": "wf-1",
            "agent_name": "flash-agent",
        }

        tool_payload = {
            "messages": [
                {
                    "role": "user",
                    "content": "You are an ITOps routing agent. Given a monitoring query, choose the best data source.",
                }
            ],
            "metadata": {},
        }
        llm_payload = {
            "messages": [
                {
                    "role": "user",
                    "content": "INSTRUCTIONS: You are an expert system-analysis agent for fault-injection runs.",
                }
            ],
            "metadata": {},
        }

        tool_result = json.loads(module.ProxyHandler._inject_metadata(json.dumps(tool_payload).encode("utf-8"), context))
        llm_result = json.loads(module.ProxyHandler._inject_metadata(json.dumps(llm_payload).encode("utf-8"), context))

        tool_md = tool_result.get("metadata", {})
        llm_md = llm_result.get("metadata", {})

        checks = {
            "tool_has_no_gt_flag": "is_ground_truth_data" not in tool_md,
            "llm_has_gt_flag": llm_md.get("is_ground_truth_data") is True,
            "llm_has_block_type": llm_md.get("gt_block_type") == "llm_analysis",
            "llm_has_fault_names": isinstance(llm_md.get("fault_names"), list) and len(llm_md.get("fault_names")) > 0,
            "llm_has_expected_output": isinstance(llm_md.get("expected_output"), str) and len(llm_md.get("expected_output")) > 0,
            "llm_has_backward_marker": llm_md.get("gt_metadata_present") is True,
        }

        for key, value in checks.items():
            print(f"{key}={value}")

        if all(checks.values()):
            print("RESULT=PASS")
            return 0

        print("RESULT=FAIL")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
