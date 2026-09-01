#!/usr/bin/env python3
"""transcribe —— C2 语音转写算子（IR-0007 AC-7 / BEH-6：口播三要素验证端）。

调用形式（C2 契约 schema/contracts/operator/*.json）：
    python main.py transcribe --contract-version 1 < request.json > response.json

inputs:  audio_path(语音文件绝对路径，禁止 URL)
         text_hint(口播源文案——fake 后端的确定性透传锚；真实后端忽略)
params:  model(后端注册表键) / language
outputs: text(转写文本)

用途：形态4 管线对 gen_tts 产物做独立转写，验证品牌名/卖点/行动号召
三要素齐全（AC-7）。算子是无状态纯函数：不联网；同输入同输出。
"""

import json
import os
import sys
import time

CONTRACT_VERSION = 1
OP = "transcribe"
OPERATOR_VERSION = "transcribe@0.1.0"


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
    p = inputs.get("audio_path")
    if not isinstance(p, str) or not p:
        raise InputError("missing_audio", "inputs.audio_path 必填（绝对路径）")
    if p.startswith(("http://", "https://")):
        raise InputError("url_forbidden", "audio_path 禁止 URL——算子不联网，传绝对路径")
    if not os.path.isabs(p):
        raise InputError("relative_path", "audio_path 必须绝对路径: %r" % p)
    if not os.path.isfile(p):
        raise InputError("audio_missing", "audio_path 不存在或不可读: %s" % p)
    hint = inputs.get("text_hint")
    if hint is not None and not isinstance(hint, str):
        raise InputError("bad_text_hint", "inputs.text_hint 必须为字符串")
    return p, hint


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
        audio_path, hint = validate_inputs(req.get("inputs") or {})
        params = req.get("params") or {}
        model = params.get("model", "fake")
        if not isinstance(model, str) or not model:
            raise InputError("bad_model", "params.model 必须非空字符串")
        if model != "fake" and not isinstance(seed, int):
            raise InputError("missing_seed",
                             "真实推理后端必须 determinism.seed（AC-3 重放条款）")
        if model == "fake" and (hint is None or not hint.strip()):
            raise InputError("missing_text_hint",
                             "fake 后端需要 inputs.text_hint（fake 音频无语义的转写联调"
                             "替身；真实链路用 whisper 从音频独立转写）")
        os.makedirs(workdir, exist_ok=True)
        from backends import REGISTRY  # 延迟导入：fake 路径零第三方依赖

        if model not in REGISTRY:
            raise InputError("unknown_model",
                             "未注册的模型后端 %r；可选: %s" % (model, ", ".join(sorted(REGISTRY))))
        out = REGISTRY[model](audio_path, hint, seed, params, workdir).transcribe()
    except InputError as e:
        fail("INPUT_ERROR", e.code, e.message)
    except ImportError as e:
        fail("RUNTIME_ERROR", "deps_missing",
             "算子镜像缺依赖（whisper 链路）: %s；fake 后端无需依赖" % e, retryable=False)
    except Exception as e:  # 契约兜底：算子必须输出结构化错误，不得裸抛
        fail("RUNTIME_ERROR", "backend_failed",
             "%s 转写失败: %s" % (req.get("params", {}).get("model"), e), retryable=True)

    wall_ms = int((time.monotonic() - started) * 1000)
    print(json.dumps(response(
        "OK",
        {"text": out["text"]},
        {"wall_ms": wall_ms, **out.get("metrics", {})},
        model_versions=out.get("model_versions"),
    ), ensure_ascii=False))


if __name__ == "__main__":
    main()
