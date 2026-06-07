#!/usr/bin/env python3
"""Tests for the validator DeepSeek dispute script."""

from __future__ import annotations

import importlib.util
import json
import pathlib
import unittest


SCRIPT_PATH = pathlib.Path(__file__).with_name("run.py")


def load_script():
    spec = importlib.util.spec_from_file_location("validator_deepseek_run", SCRIPT_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError("failed to load run.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def evidence() -> dict[str, object]:
    return {
        "marketAddress": "0x1111111111111111111111111111111111111111",
        "orderId": "12",
        "buyerAddress": "0x2222222222222222222222222222222222222222",
        "sellerAddress": "0x3333333333333333333333333333333333333333",
        "validatorAddress": "0x4444444444444444444444444444444444444444",
        "requestHash": "0xrequest",
        "deliveryHash": "0xdelivery",
        "request": "translate this text",
        "delivery": "translated text",
        "dispute": "translation is incomplete",
    }


class ValidatorDeepSeekScriptTest(unittest.TestCase):
    def setUp(self):
        self.script = load_script()

    def test_load_evidence_requires_json_object(self):
        with self.assertRaisesRegex(RuntimeError, "valid JSON"):
            self.script.load_evidence("not json")
        with self.assertRaisesRegex(RuntimeError, "must be an object"):
            self.script.load_evidence("[]")

    def test_load_evidence_requires_protocol_fields(self):
        raw = json.dumps({"orderId": "12"})

        with self.assertRaisesRegex(RuntimeError, "missing evidence field"):
            self.script.load_evidence(raw)

    def test_decide_builds_prompt_and_returns_compact_json(self):
        calls = []

        def complete(payload):
            calls.append(payload)
            self.assertEqual(payload["model"], "deepseek-chat")
            user_content = payload["messages"][1]["content"]
            self.assertIn("orderId: 12", user_content)
            self.assertIn("translate this text", user_content)
            return json.dumps(
                {
                    "releaseToSeller": False,
                    "summary": "buyer wins",
                    "reasoning": "delivery was incomplete",
                    "buyerClaim": "missing translation",
                    "sellerDeliveryAssessment": "insufficient",
                    "confidence": "high",
                }
            )

        result = json.loads(self.script.decide(evidence(), complete))

        self.assertEqual(len(calls), 1)
        self.assertFalse(result["releaseToSeller"])
        self.assertEqual(result["summary"], "buyer wins")
        self.assertEqual(result["confidence"], "high")

    def test_parse_model_decision_accepts_code_fence(self):
        content = """```json
{"releaseToSeller":true,"summary":"seller wins","reasoning":"complete delivery","buyerClaim":"bad","sellerDeliveryAssessment":"sufficient","confidence":"low"}
```"""

        decision = self.script.parse_model_decision(content)

        self.assertTrue(decision["releaseToSeller"])
        self.assertEqual(decision["summary"], "seller wins")

    def test_validate_decision_defaults_empty_confidence(self):
        decision = self.script.validate_decision(
            {
                "releaseToSeller": True,
                "summary": "seller wins",
                "reasoning": "complete delivery",
                "buyerClaim": "bad",
                "sellerDeliveryAssessment": "sufficient",
                "confidence": " ",
            }
        )

        self.assertEqual(decision["confidence"], "medium")

    def test_validate_decision_rejects_bad_release_flag(self):
        with self.assertRaisesRegex(RuntimeError, "releaseToSeller must be a boolean"):
            self.script.validate_decision(
                {
                    "releaseToSeller": "true",
                    "summary": "seller wins",
                    "reasoning": "complete delivery",
                    "buyerClaim": "bad",
                    "sellerDeliveryAssessment": "sufficient",
                    "confidence": "high",
                }
            )

    def test_validate_decision_rejects_empty_summary_or_reasoning(self):
        with self.assertRaisesRegex(RuntimeError, "summary must be non-empty"):
            self.script.validate_decision(
                {
                    "releaseToSeller": True,
                    "summary": " ",
                    "reasoning": "complete delivery",
                    "buyerClaim": "bad",
                    "sellerDeliveryAssessment": "sufficient",
                    "confidence": "high",
                }
            )

    def test_extract_choice_content_rejects_invalid_response(self):
        with self.assertRaisesRegex(RuntimeError, "invalid chat completion response"):
            self.script.extract_choice_content(b'{"choices":[]}')

    def test_extract_choice_content_returns_message_content(self):
        body = json.dumps({"choices": [{"message": {"content": "{\"releaseToSeller\":true}"}}]}).encode()

        self.assertEqual(self.script.extract_choice_content(body), '{"releaseToSeller":true}')


if __name__ == "__main__":
    unittest.main()
