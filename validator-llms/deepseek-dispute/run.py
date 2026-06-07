#!/usr/bin/env python3
"""Decide Agent Nexus validator disputes with DeepSeek."""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request


DEFAULT_DEEPSEEK_URL = "https://api.deepseek.com/chat/completions"
DEFAULT_DEEPSEEK_MODEL = "deepseek-chat"
REQUEST_TIMEOUT_SECONDS = 60

REQUIRED_EVIDENCE_FIELDS = (
    "marketAddress",
    "orderId",
    "buyerAddress",
    "sellerAddress",
    "validatorAddress",
    "requestHash",
    "deliveryHash",
    "request",
    "delivery",
    "dispute",
)

REQUIRED_DECISION_FIELDS = (
    "releaseToSeller",
    "summary",
    "reasoning",
    "buyerClaim",
    "sellerDeliveryAssessment",
    "confidence",
)


def load_evidence(raw: str) -> dict[str, object]:
    try:
        evidence = json.loads(raw)
    except json.JSONDecodeError as err:
        raise RuntimeError(f"stdin must be valid JSON: {err}") from err
    if not isinstance(evidence, dict):
        raise RuntimeError("stdin JSON must be an object")

    missing = [field for field in REQUIRED_EVIDENCE_FIELDS if field not in evidence]
    if missing:
        raise RuntimeError(f"missing evidence field(s): {', '.join(missing)}")
    return evidence


def validate_decision(value: object) -> dict[str, object]:
    if not isinstance(value, dict):
        raise RuntimeError("decision must be a JSON object")

    missing = [field for field in REQUIRED_DECISION_FIELDS if field not in value]
    if missing:
        raise RuntimeError(f"missing decision field(s): {', '.join(missing)}")
    if not isinstance(value["releaseToSeller"], bool):
        raise RuntimeError("releaseToSeller must be a boolean")

    decision: dict[str, object] = {"releaseToSeller": value["releaseToSeller"]}
    for field in REQUIRED_DECISION_FIELDS:
        if field == "releaseToSeller":
            continue
        if value[field] is None:
            text = ""
        else:
            text = str(value[field]).strip()
        if field in ("summary", "reasoning") and not text:
            raise RuntimeError(f"{field} must be non-empty")
        if field == "confidence" and not text:
            text = "medium"
        decision[field] = text

    return decision


def trim_code_fence(content: str) -> str:
    content = content.strip()
    if not content.startswith("```"):
        return content
    lines = content.splitlines()
    if len(lines) >= 3:
        return "\n".join(lines[1:-1]).strip()
    return content


def parse_model_decision(content: str) -> dict[str, object]:
    content = trim_code_fence(content)
    try:
        parsed = json.loads(content)
    except json.JSONDecodeError as err:
        raise RuntimeError(f"DeepSeek returned invalid decision JSON: {err}") from err
    return validate_decision(parsed)


def system_prompt() -> str:
    return """You are an Agent Nexus validator. Decide a digital delivery escrow dispute.
Return ONLY valid JSON with exactly these fields:
{
  "releaseToSeller": true,
  "summary": "...",
  "reasoning": "...",
  "buyerClaim": "...",
  "sellerDeliveryAssessment": "...",
  "confidence": "low|medium|high"
}
Set releaseToSeller=true if the seller substantially satisfied the buyer request and delivery standard. Set it false if the buyer should receive the escrow amount."""


def user_prompt(evidence: dict[str, object]) -> str:
    return f"""Order:
marketAddress: {evidence["marketAddress"]}
orderId: {evidence["orderId"]}
buyerAddress: {evidence["buyerAddress"]}
sellerAddress: {evidence["sellerAddress"]}
validatorAddress: {evidence["validatorAddress"]}
requestHash: {evidence["requestHash"]}
deliveryHash: {evidence["deliveryHash"]}

Buyer request:
{evidence["request"]}

Seller delivery:
{evidence["delivery"]}

Buyer dispute reason:
{evidence["dispute"]}
"""


def build_chat_request(evidence: dict[str, object], model: str) -> dict[str, object]:
    return {
        "model": model,
        "messages": [
            {"role": "system", "content": system_prompt()},
            {"role": "user", "content": user_prompt(evidence)},
        ],
        "temperature": 0,
    }


def extract_choice_content(response_body: bytes) -> str:
    try:
        response = json.loads(response_body.decode("utf-8"))
    except json.JSONDecodeError as err:
        raise RuntimeError(f"DeepSeek returned invalid response JSON: {err}") from err
    try:
        content = response["choices"][0]["message"]["content"]
    except (KeyError, IndexError, TypeError) as err:
        raise RuntimeError("DeepSeek returned an invalid chat completion response") from err
    if not isinstance(content, str) or not content.strip():
        raise RuntimeError("DeepSeek returned an empty decision")
    return content


def call_deepseek(api_key: str, url: str, payload: dict[str, object]) -> bytes:
    body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=body,
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=REQUEST_TIMEOUT_SECONDS) as response:
            return response.read()
    except urllib.error.HTTPError as err:
        detail = err.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"DeepSeek HTTP error {err.code}: {detail}") from err
    except urllib.error.URLError as err:
        raise RuntimeError(f"DeepSeek request failed: {err.reason}") from err
    except TimeoutError as err:
        raise RuntimeError("DeepSeek request timed out") from err


def decide(evidence: dict[str, object], complete) -> str:
    model = os.environ.get("VALIDATOR_LLM_MODEL", DEFAULT_DEEPSEEK_MODEL).strip() or DEFAULT_DEEPSEEK_MODEL
    content = complete(build_chat_request(evidence, model))
    decision = parse_model_decision(content)
    return json.dumps(decision, ensure_ascii=False, separators=(",", ":"))


def main() -> int:
    try:
        api_key = os.environ.get("VALIDATOR_LLM_API_KEY", "").strip()
        if not api_key:
            raise RuntimeError("VALIDATOR_LLM_API_KEY is required")
        url = os.environ.get("VALIDATOR_LLM_BASE_URL", DEFAULT_DEEPSEEK_URL).strip() or DEFAULT_DEEPSEEK_URL
        evidence = load_evidence(sys.stdin.read())

        def complete(payload: dict[str, object]) -> str:
            return extract_choice_content(call_deepseek(api_key, url, payload))

        print(decide(evidence, complete))
        return 0
    except Exception as err:
        print(str(err), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
