#!/usr/bin/env python3
"""Tests for the TAS DeepSeek script protocol."""

from __future__ import annotations

import importlib.util
import json
import pathlib
import unittest


SCRIPT_PATH = pathlib.Path(__file__).with_name("run.py")


def load_script():
    spec = importlib.util.spec_from_file_location("tas_run", SCRIPT_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError("failed to load run.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class RunProtocolTest(unittest.TestCase):
    def setUp(self):
        self.script = load_script()

    def test_need_info_then_final(self):
        calls = []

        def complete(messages):
            calls.append(messages)
            if len(calls) == 1:
                return json.dumps({"status": "need_info", "requests": ["tas.plutonic.zh"]})
            self.assertIn("TAS_PLUTONIC_zh", messages[-1]["content"])
            return json.dumps({"status": "final", "answer": "深成岩 TAS 绘图数据已整理。", "evidence": "使用 tas.plutonic.zh。"})

        result = json.loads(self.script.run_protocol("请整理深成岩 TAS 中文绘图数据", complete))

        self.assertEqual(result["answer"], "深成岩 TAS 绘图数据已整理。")
        self.assertEqual(result["evidence"], "使用 tas.plutonic.zh。")
        self.assertEqual(len(calls), 2)

    def test_unknown_info_key_fails(self):
        def complete(_messages):
            return json.dumps({"status": "need_info", "requests": ["tas.unknown"]})

        with self.assertRaisesRegex(RuntimeError, "unknown info key"):
            self.script.run_protocol("need bad data", complete)

    def test_invalid_json_fails(self):
        with self.assertRaisesRegex(RuntimeError, "invalid protocol JSON"):
            self.script.run_protocol("bad json", lambda _messages: "not json")

    def test_final_answer_with_unescaped_quotes_is_recovered(self):
        content = '{"status":"final","answer":"Use "Foidite" at (41.6,10.2)."}'

        result = json.loads(self.script.run_protocol("quoted labels", lambda _messages: content))

        self.assertEqual(result["answer"], 'Use "Foidite" at (41.6,10.2).')
        self.assertNotEqual(result["evidence"], "")

    def test_empty_final_answer_fails(self):
        def complete(_messages):
            return json.dumps({"status": "final", "answer": "  "})

        with self.assertRaisesRegex(RuntimeError, "empty final answer"):
            self.script.run_protocol("empty answer", complete)


if __name__ == "__main__":
    unittest.main()
