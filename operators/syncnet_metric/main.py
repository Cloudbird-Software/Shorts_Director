#!/usr/bin/env python3
"""syncnet_metric —— C2 口型同步质量探针算子（IR-0007 DECISION-4 / 卡 #119）。

调用形式（一个算子二选一 op——白名单见 internal/qc/assertion.go probeOps）：
    python main.py lipsync_lse_c --contract-version 1 < req.json > resp.json
    python main.py lipsync_lse_d --contract-version 1 < req.json > resp.json

inputs:  media_path(成品口播视频绝对路径，禁止 URL)
outputs: value(该 op 对应指标) / evidence_uri(指标明细 JSON)

指标口径（SyncNet 类，DECISION-4 判定锚）：
  LSE-C（confidence）：音视频嵌入相似度置信，越高口型越同步；
  LSE-D（distance）  ：音视频嵌入欧氏距离，越低口型越同步。
断言阈值在套件 expect.value 配置（可配，不在算子内写死）。
"""

import hashlib
import json
import os
import sys
import time

CONTRACT_VERSION = 1
OPS = ("lipsync_lse_c", "lipsync_lse_d")
OPERATOR_VERSION = "syncnet_metric@0.1.0"


class InputError(Exception):
    """坏输入，可由上游重传修复。"""

    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def response(op, status, outputs, metrics, model_versions=None, error=None):
    resp = {
        "contract_version": CONTRACT_VERSION,
        "op": op,
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


def fail(op, status, code, message, retryable=False):
    print(json.dumps(response(op, status, {}, {"wall_ms": 0},
                              error={"code": code, "message": message,
                                     "retryable": retryable}),
                     ensure_ascii=False))
    sys.exit(0)


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
    return p


def main():
    argv = sys.argv[1:]
    op = argv[0] if argv else ""
    if op not in OPS or "--contract-version" not in argv:
        fail(op or "lipsync_lse_c", "INPUT_ERROR", "bad_cli",
             "调用形式: main.py <%s> --contract-version %d" % ("|".join(OPS), CONTRACT_VERSION))
    try:
        req = json.load(sys.stdin)
    except json.JSONDecodeError as e:
        fail(op, "INPUT_ERROR", "bad_json", "请求不是合法 JSON: %s" % e)
    if req.get("contract_version") != CONTRACT_VERSION:
        fail(op, "INPUT_ERROR", "bad_contract_version",
             "contract_version 必须 %d" % CONTRACT_VERSION)
    workdir = req.get("workdir")
    if not isinstance(workdir, str) or not workdir:
        fail(op, "INPUT_ERROR", "missing_workdir", "workdir 必填（内容寻址）")
    seed = (req.get("determinism") or {}).get("seed")

    started = time.monotonic()
    try:
        media_path = validate_inputs(req.get("inputs") or {})
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
        out = REGISTRY[model](media_path, op, seed, workdir).measure()
    except InputError as e:
        fail(op, "INPUT_ERROR", e.code, e.message)
    except ImportError as e:
        fail(op, "RUNTIME_ERROR", "deps_missing",
             "算子镜像缺依赖（SyncNet 链路）: %s；fake 后端无需依赖" % e, retryable=False)
    except Exception as e:  # 契约兜底：算子必须输出结构化错误，不得裸抛
        fail(op, "RUNTIME_ERROR", "backend_failed",
             "%s 口型指标计算失败: %s" % (req.get("params", {}).get("model"), e), retryable=True)

    wall_ms = int((time.monotonic() - started) * 1000)
    print(json.dumps(response(
        op, "OK",
        {"value": out["value"], "evidence_uri": out["evidence_uri"]},
        {"wall_ms": wall_ms, **out.get("metrics", {})},
        model_versions=out.get("model_versions"),
    ), ensure_ascii=False))


if __name__ == "__main__":
    main()
