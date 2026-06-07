#!/usr/bin/env python3
"""Ask DeepSeek to organize built-in TAS diagram reference data."""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request


DEEPSEEK_URL = "https://api.deepseek.com/chat/completions"
DEEPSEEK_MODEL = "deepseek-v4-flash"
REQUEST_TIMEOUT_SECONDS = 60


def line(x1: float, y1: float, x2: float, y2: float) -> tuple[float, float, float, float]:
    return (x1, y1, x2, y2)


def label(x: float, y: float, text: str, fontsize: int) -> tuple[float, float, str, int]:
    return (x, y, text, fontsize)


TAS_VOLCANIC_LINES = [
    line(41.0, -1.0, 41.0, 7.0),
    line(41.0, 7.0, 52.5, 14.0),
    line(45.0, -1.0, 45.0, 5.0),
    line(45.0, 5.0, 61.0, 13.5),
    line(52.0, -1.0, 52.0, 5.0),
    line(52.0, 5.0, 69.0, 8.0),
    line(41.0, 3.0, 45.0, 3.0),
    line(45.0, 5.0, 52.0, 5.0),
    line(45.0, 9.4, 49.4, 7.3),
    line(48.4, 11.5, 53.0, 9.3),
    line(52.5, 14.0, 57.6, 11.7),
    line(49.4, 7.3, 52.0, 5.0),
    line(53.0, 9.3, 57.0, 5.9),
    line(57.6, 11.7, 63.0, 7.0),
    line(57.0, 5.9, 57.0, -0.1),
    line(63.0, 7.0, 63.0, 1.0),
    line(69.0, 8.0, 69.0, 12.0),
    line(69.0, 8.0, 73.0, 4.0),
]

TAS_VOLCANIC_LABELS_EN = [
    label(41.6, 10.2, "Foidite", 10),
    label(41.5, 1.9, "Picrobasalt", 9),
    label(41.6, 6.1, "Tephrite/Basanite", 9),
    label(46.1, 9.0, "Phonotephrite", 9),
    label(51.3, 12.0, "Tephriphonolite", 9),
    label(47.9, 5.6, "Trachybasalt", 9),
    label(51.5, 7.0, "Basaltic Trachyandesite", 7),
    label(56.5, 8.0, "Trachyandesite", 8),
    label(58.5, 13.0, "Phonolite", 10),
    label(47.4, 3.2, "Basalt", 9),
    label(53.7, 3.2, "Basaltic Andesite", 7),
    label(59.4, 3.2, "Andesite", 8),
    label(65.6, 4.2, "Dacite", 9),
    label(70.9, 8.1, "Rhyolite", 9),
    label(61.8, 11.1, "Trachyte/Trachydacite", 7),
]

TAS_VOLCANIC_LABELS_ZH = [
    label(41.6, 10.2, "似长石", 10),
    label(41.5, 1.9, "苦橄玄武岩", 9),
    label(41.6, 6.1, "碱玄岩/碧玄岩", 9),
    label(46.0, 9.0, "响岩质碱玄武岩", 9),
    label(51.1, 12.0, "碱玄武响岩", 9),
    label(47.8, 5.6, "粗面玄武岩", 9),
    label(51.3, 7.0, "玄武粗安岩", 8),
    label(56.4, 8.0, "粗安岩", 9),
    label(58.4, 13.0, "响岩", 10),
    label(47.4, 3.2, "玄武岩", 9),
    label(53.6, 3.2, "玄武安山岩", 8),
    label(59.3, 3.2, "安山岩", 9),
    label(65.5, 4.2, "英安岩", 9),
    label(70.7, 8.1, "流纹岩", 9),
    label(61.5, 11.1, "粗面岩/粗面英安岩", 8),
]

TAS_PLUTONIC_LINES = [
    line(37.0, 3.0, 35.0, 9.0),
    line(35.0, 9.0, 37.0, 14.0),
    line(37.0, 14.0, 52.5, 18.0),
    line(52.5, 18.0, 57.0, 18.0),
    line(57.0, 18.0, 63.0, 16.2),
    line(71.8, 13.5, 63.0, 16.2),
    line(71.8, 13.5, 85.9, 6.8),
    line(87.5, 4.7, 85.9, 6.8),
    line(87.5, 4.7, 77.3, 0.0),
    line(77.3, 0.0, 69.0, 8.0),
    line(69.0, 8.0, 71.8, 13.5),
    line(69.0, 8.0, 63.0, 7.0),
    line(57.0, 5.9, 63.0, 7.0),
    line(57.0, 5.9, 52.0, 5.0),
    line(45.0, 5.0, 52.0, 5.0),
    line(45.0, 5.0, 49.4, 7.3),
    line(53.0, 9.3, 49.4, 7.3),
    line(53.0, 9.3, 57.6, 11.7),
    line(61.0, 13.5, 57.6, 11.7),
    line(61.0, 13.5, 63.0, 16.2),
    line(71.8, 13.5, 61.0, 8.6),
    line(63.0, 7.0, 57.6, 11.7),
    line(52.0, 5.0, 49.4, 7.3),
    line(57.0, 5.9, 53.0, 9.3),
    line(45.0, 5.0, 45.0, 0.0),
    line(37.0, 3.0, 45.0, 3.0),
    line(41.0, 3.0, 41.0, 7.0),
    line(45.0, 9.4, 41.0, 7.0),
    line(45.0, 9.4, 48.4, 11.5),
    line(52.5, 14.0, 48.4, 11.5),
    line(52.5, 14.0, 57.6, 11.7),
    line(49.4, 7.3, 45.0, 9.4),
    line(53.0, 9.3, 48.4, 11.5),
    line(52.5, 18.0, 52.5, 14.0),
    line(52.0, 5.0, 52.0, 0.0),
    line(57.0, 5.9, 57.0, 0.0),
    line(63.0, 7.0, 63.0, 0.0),
    line(41.0, 3.0, 41.0, 0.0),
]

TAS_PLUTONIC_LABELS_EN = [
    label(38.3, 10.2, "Foid-bearing Plutonic Rock", 8),
    label(41.9, 1.9, "Gabbroic Peridotite", 7),
    label(42.6, 5.9, "Monzogabbro", 8),
    label(47.9, 9.0, "Foid Monzodiorite", 7),
    label(50.9, 11.5, "Foid Monzosyenite", 7),
    label(47.7, 5.6, "Monzogabbro", 8),
    label(50.4, 7.1, "Monzodiorite", 8),
    label(56.2, 8.0, "Monzonite", 8),
    label(54.5, 15.0, "Foid Syenite", 8),
    label(47.1, 3.2, "Gabbro", 8),
    label(53.2, 3.2, "Gabbrodiorite", 7),
    label(58.0, 3.2, "Diorite", 8),
    label(65.2, 4.2, "Granodiorite", 7),
    label(76.0, 3.2, "Granite", 8),
    label(62.3, 12.1, "Syenite", 8),
    label(63.4, 9.0, "Quartz Syenite", 7),
]

TAS_PLUTONIC_LABELS_ZH = [
    label(39.4, 10.2, "似长深成岩", 9),
    label(42.9, 1.9, "橄榄辉长岩", 8),
    label(43.8, 5.9, "副二长辉长岩", 8),
    label(49.1, 9.0, "似长石二长闪长岩", 8),
    label(51.9, 11.5, "似长二长正长岩", 8),
    label(48.8, 5.6, "二长辉长岩", 8),
    label(51.5, 7.1, "二长闪长岩", 8),
    label(57.3, 8.0, "二长岩", 9),
    label(55.3, 15.0, "似长正长岩", 8),
    label(48.2, 3.2, "辉长岩", 9),
    label(54.2, 3.2, "辉长闪长岩", 8),
    label(59.0, 3.2, "闪长岩", 9),
    label(66.0, 4.2, "花岗闪长岩", 8),
    label(76.7, 3.2, "花岗岩", 9),
    label(63.1, 12.1, "正长岩", 9),
    label(64.2, 9.0, "石英正长岩", 8),
]


def format_diagram(
    name: str,
    language: str,
    lines: list[tuple[float, float, float, float]],
    labels: list[tuple[float, float, str, int]],
    xlim: tuple[float, float],
    ylim: tuple[float, float],
) -> str:
    output = [
        f"diagram: {name}",
        f"language: {language}",
        "title: TAS",
        "x_axis: SiO2",
        "y_axis: Na2O + K2O",
        f"x_range: {xlim[0]} to {xlim[1]}",
        f"y_range: {ylim[0]} to {ylim[1]}",
        "",
        "boundary_lines:",
    ]
    output.extend(f"- ({x1:g},{y1:g}) -> ({x2:g},{y2:g})" for x1, y1, x2, y2 in lines)
    output.append("")
    output.append("labels:")
    output.extend(f"- {text} @ ({x:g},{y:g}), fontsize={fontsize}" for x, y, text, fontsize in labels)
    return "\n".join(output)


def build_info_blocks() -> dict[str, str]:
    blocks = {
        "tas.volcanic.en": format_diagram(
            "TAS_VOLCANIC_en",
            "en",
            TAS_VOLCANIC_LINES,
            TAS_VOLCANIC_LABELS_EN,
            (35.0, 80.0),
            (0.0, 16.0),
        ),
        "tas.volcanic.zh": format_diagram(
            "TAS_VOLCANIC_zh",
            "zh",
            TAS_VOLCANIC_LINES,
            TAS_VOLCANIC_LABELS_ZH,
            (35.0, 80.0),
            (0.0, 16.0),
        ),
        "tas.plutonic.en": format_diagram(
            "TAS_PLUTONIC_en",
            "en",
            TAS_PLUTONIC_LINES,
            TAS_PLUTONIC_LABELS_EN,
            (30.0, 90.0),
            (0.0, 20.0),
        ),
        "tas.plutonic.zh": format_diagram(
            "TAS_PLUTONIC_zh",
            "zh",
            TAS_PLUTONIC_LINES,
            TAS_PLUTONIC_LABELS_ZH,
            (30.0, 90.0),
            (0.0, 20.0),
        ),
    }
    blocks["tas.all"] = "\n\n".join(blocks[key] for key in sorted(blocks))
    return blocks


INFO_BLOCKS = build_info_blocks()
INFO_KEYS = sorted(INFO_BLOCKS)


def build_messages(request: str) -> list[dict[str, str]]:
    system_prompt = (
        "You are a TAS diagram drawing assistant. You must answer with one JSON object "
        "and no extra text. If you need TAS reference data before giving the final answer, "
        'return {"status":"need_info","requests":["info.key"]}. The requests array may '
        "contain only exact keys from the available info key list. Never put questions, "
        "free text, or explanations in requests. If the buyer asks a broad or ambiguous "
        "TAS question, request the closest TAS data block first, then give a practical "
        'final answer. If you can answer the buyer, return {"status":"final","answer":'
        '"plain text answer","evidence":"plain text seller evidence for validator"}. '
        "Use only the supporting TAS data provided by the script for boundary lines, "
        "axis ranges, labels, label positions, and font sizes. Do not invent TAS "
        "geometry. Match the buyer language when it is clear. The final answer text "
        "and evidence text must not be JSON. Evidence should explain what reference "
        "data was used and why the answer is consistent with the buyer request."
    )
    user_prompt = (
        "Buyer request:\n"
        f"{request.strip()}\n\n"
        "Available info keys:\n"
        + "\n".join(f"- {key}" for key in INFO_KEYS)
    )
    return [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": user_prompt},
    ]


def chat_completion_content(api_key: str, messages: list[dict[str, str]]) -> str:
    body = {
        "model": DEEPSEEK_MODEL,
        "messages": messages,
        "response_format": {"type": "json_object"},
        "stream": False,
    }
    encoded_body = json.dumps(body, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        DEEPSEEK_URL,
        data=encoded_body,
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        },
        method="POST",
    )

    try:
        with urllib.request.urlopen(request, timeout=REQUEST_TIMEOUT_SECONDS) as response:
            raw_response = response.read()
    except urllib.error.HTTPError as err:
        detail = err.read().decode("utf-8", errors="replace")[:1000]
        raise RuntimeError(f"DeepSeek HTTP error {err.code}: {detail}") from err
    except urllib.error.URLError as err:
        raise RuntimeError(f"DeepSeek request failed: {err.reason}") from err
    except TimeoutError as err:
        raise RuntimeError("DeepSeek request timed out") from err

    try:
        payload = json.loads(raw_response.decode("utf-8"))
        answer = payload["choices"][0]["message"]["content"]
    except (KeyError, IndexError, TypeError, json.JSONDecodeError) as err:
        raise RuntimeError("DeepSeek returned an invalid chat completion response") from err

    answer = str(answer).strip()
    if not answer:
        raise RuntimeError("DeepSeek returned an empty answer")
    return answer


def parse_instruction(content: str) -> dict[str, object]:
    text = content.strip()
    if text.startswith("```"):
        lines = text.splitlines()
        if lines and lines[0].startswith("```"):
            lines = lines[1:]
        if lines and lines[-1].strip() == "```":
            lines = lines[:-1]
        text = "\n".join(lines).strip()

    try:
        instruction = json.loads(text)
    except json.JSONDecodeError as err:
        fallback = parse_final_answer_fallback(text)
        if fallback is not None:
            return fallback
        raise RuntimeError("DeepSeek returned invalid protocol JSON") from err

    if not isinstance(instruction, dict):
        raise RuntimeError("DeepSeek protocol response must be a JSON object")
    return instruction


def parse_final_answer_fallback(text: str) -> dict[str, object] | None:
    prefix = '{"status":"final","answer":"'
    suffix = '"}'
    if not text.startswith(prefix) or not text.endswith(suffix):
        return None

    answer = text[len(prefix):-len(suffix)]
    answer = answer.replace("\\n", "\n").replace('\\"', '"').replace("\\/", "/")
    return {"status": "final", "answer": answer}


def format_requested_info(requests: list[str]) -> str:
    unknown = [key for key in requests if key not in INFO_BLOCKS]
    if unknown:
        raise RuntimeError(f"DeepSeek requested unknown info key: {', '.join(unknown)}")

    return "\n\n".join(
        [
            f"Info key: {key}\n{INFO_BLOCKS[key]}"
            for key in requests
        ]
    )


def run_protocol(request_text: str, complete: callable) -> str:
    messages = build_messages(request_text)

    while True:
        content = complete(messages)
        instruction = parse_instruction(content)
        status = instruction.get("status")

        if status == "final":
            answer = str(instruction.get("answer", "")).strip()
            if not answer:
                raise RuntimeError("DeepSeek returned an empty final answer")
            evidence = str(instruction.get("evidence", "")).strip()
            if not evidence:
                evidence = "The answer was generated from the built-in TAS reference data requested during this script run."
            return json.dumps({"answer": answer, "evidence": evidence}, ensure_ascii=False)

        if status != "need_info":
            raise RuntimeError("DeepSeek protocol status must be need_info or final")

        requests = instruction.get("requests")
        if not isinstance(requests, list) or not requests:
            raise RuntimeError("DeepSeek need_info response must include requests")
        request_keys = []
        for key in requests:
            if not isinstance(key, str):
                raise RuntimeError("DeepSeek info requests must be strings")
            request_keys.append(key)

        messages.append({"role": "assistant", "content": content})
        messages.append(
            {
                "role": "user",
                "content": "Requested supporting TAS data:\n" + format_requested_info(request_keys),
            }
        )


def ask_deepseek(api_key: str, request_text: str) -> str:
    return run_protocol(
        request_text,
        lambda messages: chat_completion_content(api_key, messages),
    )


def main() -> int:
    request = sys.stdin.read()
    if not request.strip():
        print("No request provided.", file=sys.stderr)
        return 2

    api_key = os.environ.get("SELLER_LLM_API_KEY", "").strip()
    if not api_key:
        print("SELLER_LLM_API_KEY is required.", file=sys.stderr)
        return 2

    try:
        print(ask_deepseek(api_key, request))
    except RuntimeError as err:
        print(str(err), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
