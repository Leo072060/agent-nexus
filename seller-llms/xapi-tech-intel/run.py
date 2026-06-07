#!/usr/bin/env python3
"""Generate demo technical intelligence answers from fixed sources with DeepSeek."""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone
from typing import Any


DEFAULT_DEEPSEEK_URL = "https://api.deepseek.com/chat/completions"
DEFAULT_DEEPSEEK_MODEL = "deepseek-chat"
REQUEST_TIMEOUT_SECONDS = 60

FIXED_SOURCES = [
    {
        "organization": "北大区块链协会",
        "topic": "近期活动",
        "activity": "ETH Beijing 黑客松比赛",
        "highlight": "Vitalik 远程连线参与/分享",
        "sourceType": "demo_fixed_source",
    }
]


class DeepSeekError(RuntimeError):
    pass


def now_iso() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def build_messages(request_text: str) -> list[dict[str, str]]:
    fixed_sources = json.dumps(FIXED_SOURCES, ensure_ascii=False, indent=2)
    return [
        {
            "role": "system",
            "content": (
                "你是 Agent Nexus 演示环境里的技术情报 seller。"
                "你必须只基于用户消息和固定信息源回答，不要编造日期、地点、嘉宾、链接、报名方式或其他未给出的细节。"
                "回答必须是中文，语气自然、可信、适合客户演示。"
                "如果用户询问最近北大区块链协会有什么活动，重点说明 ETH Beijing 黑客松比赛，以及 Vitalik 远程连线参与/分享。"
            ),
        },
        {
            "role": "user",
            "content": (
                "固定信息源如下：\n"
                f"{fixed_sources}\n\n"
                "客户问题：\n"
                f"{request_text}\n\n"
                "请生成一段简洁回答，2-4 句话即可。"
            ),
        },
    ]


def call_deepseek(api_key: str, url: str, model: str, request_text: str) -> str:
    payload: dict[str, Any] = {
        "model": model,
        "messages": build_messages(request_text),
        "temperature": 0.2,
    }
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
            response_body = response.read()
    except urllib.error.HTTPError as err:
        detail = err.read().decode("utf-8", errors="replace")
        raise DeepSeekError(f"DeepSeek HTTP error {err.code}: {detail}") from err
    except urllib.error.URLError as err:
        raise DeepSeekError(f"DeepSeek request failed: {err.reason}") from err
    except TimeoutError as err:
        raise DeepSeekError("DeepSeek request timed out") from err

    try:
        parsed = json.loads(response_body.decode("utf-8"))
        content = parsed["choices"][0]["message"]["content"]
    except (json.JSONDecodeError, KeyError, IndexError, TypeError) as err:
        raise DeepSeekError("DeepSeek returned an invalid chat completion response") from err

    if not isinstance(content, str) or not content.strip():
        raise DeepSeekError("DeepSeek returned an empty answer")
    return content.strip()


def fallback_guard(answer: str) -> str:
    required = ("ETH Beijing", "黑客松", "Vitalik", "远程")
    if all(token in answer for token in required):
        return answer
    return (
        "最近北大区块链协会相关的重点活动是 ETH Beijing 黑客松比赛。"
        "这次活动还有一个很适合演示的亮点：Vitalik 远程连线参与/分享，"
        "可以作为近期区块链技术社区活动的代表案例来介绍。"
    )


def build_evidence(request_text: str, model: str, generated_at: str) -> str:
    evidence = {
        "seller": "xapi-tech-intel",
        "mode": "demo_fixed_source_with_deepseek",
        "generatedAt": generated_at,
        "request": request_text,
        "model": model,
        "sourcePolicy": "demo 固定信息源；不做实时 xapi 检索；不编造固定信息源以外的活动细节。",
        "fixedSources": FIXED_SOURCES,
    }
    return json.dumps(evidence, ensure_ascii=False, separators=(",", ":"))


def run(request_text: str, api_key: str, url: str, model: str) -> str:
    generated_at = now_iso()
    answer = fallback_guard(call_deepseek(api_key, url, model, request_text))
    evidence = build_evidence(request_text, model, generated_at)
    return json.dumps({"answer": answer, "evidence": evidence}, ensure_ascii=False)


def main() -> int:
    request_text = sys.stdin.read().strip()
    if not request_text:
        print("No request provided.", file=sys.stderr)
        return 2

    api_key = os.environ.get("SELLER_LLM_API_KEY", "").strip()
    if not api_key:
        print("SELLER_LLM_API_KEY is required.", file=sys.stderr)
        return 2

    url = os.environ.get("SELLER_LLM_BASE_URL", DEFAULT_DEEPSEEK_URL).strip() or DEFAULT_DEEPSEEK_URL
    model = os.environ.get("SELLER_LLM_MODEL", DEFAULT_DEEPSEEK_MODEL).strip() or DEFAULT_DEEPSEEK_MODEL

    try:
        print(run(request_text, api_key, url, model))
    except DeepSeekError as err:
        print(str(err), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
