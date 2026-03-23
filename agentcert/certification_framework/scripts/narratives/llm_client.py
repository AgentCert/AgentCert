"""
Phase 3 LLM Client Utility.

Provides a thin wrapper around Azure OpenAI for all Phase 3 LLM calls.
Handles: .env loading, client init, retry with exponential backoff,
token tracking, and structured JSON schema enforcement.
"""

import copy
import json
import os
import time
from pathlib import Path
from typing import Type

from dotenv import load_dotenv
from openai import AzureOpenAI
from pydantic import BaseModel

# Load .env from certification_framework root
_ENV_PATH = Path(__file__).resolve().parent.parent.parent / ".env"
load_dotenv(_ENV_PATH)


def get_client() -> AzureOpenAI:
    """Create and return an Azure OpenAI client from env vars."""
    return AzureOpenAI(
        api_key=os.environ["AZURE_OPENAI_API_KEY"],
        azure_endpoint=os.environ["AZURE_OPENAI_ENDPOINT"],
        api_version=os.getenv("AZURE_OPENAI_API_VERSION", "2024-12-01-preview"),
    )


def _prepare_strict_schema(schema: dict) -> dict:
    """Recursively patch a Pydantic JSON schema for OpenAI strict mode."""
    schema = copy.deepcopy(schema)

    def process(obj):
        if not isinstance(obj, dict):
            return obj
        if "$ref" in obj:
            return {"$ref": obj["$ref"]}
        if "properties" in obj:
            obj["required"] = list(obj["properties"].keys())
            obj["additionalProperties"] = False
            for key in obj["properties"]:
                obj["properties"][key] = process(obj["properties"][key])
        if "$defs" in obj:
            for name in obj["$defs"]:
                obj["$defs"][name] = process(obj["$defs"][name])
        if "items" in obj:
            obj["items"] = process(obj["items"])
        for key in ("anyOf", "oneOf", "allOf"):
            if key in obj:
                obj[key] = [process(item) for item in obj[key]]
        return obj

    return process(schema)


def call_llm(
    client: AzureOpenAI,
    system_prompt: str,
    user_prompt: str,
    *,
    deployment: str | None = None,
    temperature: float = 0.3,
    max_tokens: int = 4096,
    retries: int = 3,
    expect_json: bool = True,
    response_schema: Type[BaseModel] | None = None,
) -> dict:
    """
    Send a prompt to Azure OpenAI and return the result.

    Args:
        client: AzureOpenAI client instance.
        system_prompt: System message setting the LLM role.
        user_prompt: User message with context and task.
        deployment: Azure deployment name (defaults to env var).
        temperature: Sampling temperature.
        max_tokens: Max response tokens.
        retries: Number of retry attempts on failure.
        expect_json: If True, parse response as JSON.
        response_schema: If provided, enforce this Pydantic model as a
            strict JSON schema via OpenAI structured output.

    Returns:
        {
            "content": <validated Pydantic model if response_schema, else parsed JSON or raw string>,
            "model": <model name>,
            "tokens_used": <total tokens>,
            "raw_response": <raw response text>,
        }

    Raises:
        RuntimeError: If all retries are exhausted.
    """
    deployment = deployment or os.getenv("AZURE_OPENAI_CHAT_DEPLOYMENT_NAME", "gpt-4o")

    messages = [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": user_prompt},
    ]

    # Build response_format
    if response_schema is not None:
        schema = _prepare_strict_schema(response_schema.model_json_schema())
        response_format = {
            "type": "json_schema",
            "json_schema": {
                "name": response_schema.__name__,
                "schema": schema,
                "strict": True,
            },
        }
    elif expect_json:
        response_format = {"type": "json_object"}
    else:
        response_format = None

    last_error = None
    for attempt in range(retries):
        try:
            response = client.chat.completions.create(
                model=deployment,
                messages=messages,
                temperature=temperature,
                max_tokens=max_tokens,
                response_format=response_format,
            )

            raw_text = response.choices[0].message.content.strip()
            total_tokens = response.usage.total_tokens if response.usage else 0
            model_name = response.model or deployment

            if response_schema is not None:
                content = response_schema.model_validate(json.loads(raw_text))
            elif expect_json:
                content = json.loads(raw_text)
            else:
                content = raw_text

            return {
                "content": content,
                "model": model_name,
                "tokens_used": total_tokens,
                "raw_response": raw_text,
            }

        except Exception as e:
            last_error = e
            if attempt < retries - 1:
                wait = 2 ** attempt  # 1s, 2s, 4s
                time.sleep(wait)

    raise RuntimeError(
        f"LLM call failed after {retries} attempts: {last_error}"
    )
