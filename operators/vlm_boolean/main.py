#!/usr/bin/env python3
"""vlm_boolean —— C2 布尔评审算子（IR-0007 AC-8 / BEH-7：裁判校准的探针端）。

调用形式（C2 契约 schema/contracts/operator/*.json）：
    python main.py vlm_boolean --contract-version 1 < request.json > response.json

inputs:  media_path(图像/视频文件绝对路径，禁止 URL)
         question(布尔问题，非空)
         answer_hint(可选 bool——fake 后端的确定性透传锚；真实后端忽略)
params:  model(后端注册表键：fake|qwen-vl)
outputs: answer(bool) / evidence(str，判定依据描述)

用途：评审探针对生成物可用性做布尔判定（L1 语义级），供裁判校准
（卡 #121：探针判定 vs 人工标注 → 混淆矩阵 + 一致率）。算子是无状态
纯函数：不联网；同输入同输出。
"""

import json
import os
import sys
import time

CONTRACT_VERSION = 1
OP = "vlm_boolean"
OPERATOR_VERSION = "vlm_boolean@0.1.0"


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


MEDIA_EXTS = {
    ".png", ".jpg", ".jpeg", ".webp", ".bmp",  # 图像
    ".mp4", ".mov", ".mkv", ".webm", ".avi",   # 视频
}


def validate_inputs(inputs):
    p = inputs.get("media_path")
    if not isinstance(p, str) or not p:
        raise InputError("missing_media", "inputs.media_path 必填（绝对路径）")
    if p.startswith(("http://", "https://")):
        raise InputError("url_forbidden", "media_path 禁止 URL——算子不联网，传绝对路径")
    if not os.path.isabs(p):
        raise InputError("relative_path", "media_path 必须绝对路径: %r" % p)
    if not os.path.isfile(p):
        raise InputError("media_missing", "media_path 不存在或不可读: %s" % p)
    if os.path.splitext(p)[1].lower() not in MEDIA_EXTS:
        raise InputError("bad_media_type",
                         "media_path 扩展名不在图像/视频白名单: %s" % p)
    q = inputs.get("question")
    if not isinstance(q, str) or not q.strip():
        raise InputError("missing_question", "inputs.question 必填（布尔问题）")
    hint = inputs.get("answer_hint")
    if hint is not None and not isinstance(hint, bool):
        raise InputError("bad_answer_hint", "inputs.answer_hint 必须为 bool")
    return p, q, hint


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
        media_path, question, hint = validate_inputs(req.get("inputs") or {})
        params = req.get("params") or {}
        model = params.get("model", "fake")
        if not isinstance(model, str) or not model:
            raise InputError("bad_model", "params.model 必须非空字符串")
        if model != "fake" and not isinstance(seed, int):
            raise InputError("missing_seed",
                             "真实推理后端必须 determinism.seed（AC-3 重放条款）")
        os.makedirs(workdir, exist_ok=True)
        from backends import REGISTRY  # 延迟导入：fake 路径零第三方依赖

        if model not in REGISTRY:
            raise InputError("unknown_model",
                             "未注册的模型后端 %r；可选: %s" % (model, ", ".join(sorted(REGISTRY))))
        out = REGISTRY[model](media_path, question, hint, seed, params,
                              workdir).judge()
    except InputError as e:
        fail("INPUT_ERROR", e.code, e.message)
    except ImportError as e:
        fail("RUNTIME_ERROR", "deps_missing",
             "算子镜像缺依赖（VLM 链路）: %s；fake 后端无需依赖" % e, retryable=False)
    except Exception as e:  # 契约兜底：算子必须输出结构化错误，不得裸抛
        fail("RUNTIME_ERROR", "backend_failed",
             "%s 评审失败: %s" % (req.get("params", {}).get("model"), e), retryable=True)

    wall_ms = int((time.monotonic() - started) * 1000)
    print(json.dumps(response(
        "OK",
        {"answer": out["answer"], "evidence": out["evidence"]},
        {"wall_ms": wall_ms, **out.get("metrics", {})},
        model_versions=out.get("model_versions"),
    ), ensure_ascii=False))


if __name__ == "__main__":
    main()
