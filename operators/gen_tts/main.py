#!/usr/bin/env python3
"""gen_tts —— C2 文本转语音算子（IR-0007 AC-7 前置 / DECISION-4）。

调用形式（C2 契约 schema/contracts/operator/*.json）：
    python main.py gen_tts --contract-version 1 < request.json > response.json

inputs:  text(口播文案，含品牌名/卖点/行动号召三要素)
params:  model(后端注册表键) / voice(音色，默认音色免克隆) / speed / sample_rate
outputs: audio_path / content_hash(产物 sha256) / duration_sec

算子是无状态纯函数：不联网、不知道数据库/租户/业务；
同输入（含 determinism.seed）必须同输出（AC-3 重放条款）。
"""

import json
import sys
import time

CONTRACT_VERSION = 1
OP = "gen_tts"
OPERATOR_VERSION = "gen_tts@0.1.0"


class InputError(Exception):
    """坏输入，可由上游重传修复。"""

    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def response(status, outputs, metrics, model_versions=None, error=None):
    resp = {
        "contract_version": CONTRACT_VERSION,
        "op": OP,
        "status": status,
        "outputs": outputs,
        "metrics": metrics,
        "operator_version": OPERATOR_VERSION,
    }
    if model_versions:
        resp["model_versions"] = model_versions
    if error is not None:
        resp["error"] = error
    return resp


def fail(status, code, message, retryable=False):
    print(json.dumps(response(status, {}, {"wall_ms": 0},
                              error={"code": code, "message": message,
                                     "retryable": retryable}),
                     ensure_ascii=False))
    sys.exit(0)


def validate_inputs(inputs):
    text = inputs.get("text")
    if not isinstance(text, str) or not text.strip():
        raise InputError("missing_text", "inputs.text 必填非空（口播文案）")
    return text


def main():
    argv = sys.argv[1:]
    if argv[:1] != [OP] or "--contract-version" not in argv:
        fail("INPUT_ERROR", "bad_cli",
             "调用形式: main.py %s --contract-version %d" % (OP, CONTRACT_VERSION))
    try:
        req = json.load(sys.stdin)
    except json.JSONDecodeError as e:
        fail("INPUT_ERROR", "bad_json", "请求不是合法 JSON: %s" % e)
    if req.get("contract_version") != CONTRACT_VERSION:
        fail("INPUT_ERROR", "bad_contract_version",
             "contract_version 必须 %d" % CONTRACT_VERSION)
    workdir = req.get("workdir")
    if not isinstance(workdir, str) or not workdir:
        fail("INPUT_ERROR", "missing_workdir", "workdir 必填（内容寻址）")
    seed = (req.get("determinism") or {}).get("seed")

    started = time.monotonic()
    try:
        text = validate_inputs(req.get("inputs") or {})
        params = req.get("params") or {}
        model = params.get("model", "fake")
        if not isinstance(model, str) or not model:
            raise InputError("bad_model", "params.model 必须非空字符串")
        if model != "fake" and not isinstance(seed, int):
            raise InputError("missing_seed",
                             "真实生成后端必须 determinism.seed（AC-3 重放条款）")
        import os

        os.makedirs(workdir, exist_ok=True)
        from backends import REGISTRY  # 延迟导入：fake 路径零第三方依赖

        if model not in REGISTRY:
            raise InputError("unknown_model",
                             "未注册的模型后端 %r；可选: %s" % (model, ", ".join(sorted(REGISTRY))))
        out = REGISTRY[model](text, seed, params, workdir).generate()
    except InputError as e:
        fail("INPUT_ERROR", e.code, e.message)
    except ImportError as e:
        fail("RUNTIME_ERROR", "deps_missing",
             "算子镜像缺依赖（CosyVoice 链路）: %s；fake 后端无需依赖" % e, retryable=False)
    except Exception as e:  # 契约兜底：算子必须输出结构化错误，不得裸抛
        fail("RUNTIME_ERROR", "backend_failed",
             "%s 语音合成失败: %s" % (req.get("params", {}).get("model"), e), retryable=True)

    wall_ms = int((time.monotonic() - started) * 1000)
    print(json.dumps(response(
        "OK",
        {"audio_path": out["audio_path"], "content_hash": out["content_hash"],
         "duration_sec": out["duration_sec"]},
        {"wall_ms": wall_ms, **out.get("metrics", {})},
        model_versions=out.get("model_versions"),
    ), ensure_ascii=False))


if __name__ == "__main__":
    main()
