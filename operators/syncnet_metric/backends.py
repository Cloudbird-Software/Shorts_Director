"""syncnet_metric 模型后端注册表——可插拔（换模型不改 C2 契约）。

真实后端：SyncNet（joonson/syncnet_python，BSD-3）音视频联合嵌入，
LSE-C 置信 / LSE-D 距离（DECISION-4 判定锚，不引入人工逐帧评审）。
fake 后端零依赖：由成品文件 sha256 派生确定指标值（同输入恒同输出）。
"""

import hashlib
import json
import os


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


class FakeBackend:
    """确定性假后端：指标值 = 8 hex（文件摘要前缀）的模运算派生。

    值域：LSE-C ∈ [6.0, 9.9]（越高越好）；LSE-D ∈ [5.5, 8.4]（越低越好）。
    仅供无 GPU 环境的断言链路联调——套件阈值按值域上界收紧即可控负例。
    """

    def __init__(self, media_path, op, seed, workdir):
        self.media_path, self.op = media_path, op
        self.seed = seed if isinstance(seed, int) else 0
        self.workdir = workdir

    def measure(self):
        h = int(sha256_file(self.media_path)[:8], 16)
        if self.op == "lipsync_lse_c":
            value = round(6.0 + (h % 40) / 10.0, 2)  # [6.0, 9.9]
        else:
            value = round(5.5 + (h % 30) / 10.0, 2)  # [5.5, 8.4]
        evidence = os.path.join(self.workdir, "evidence_%s.json" % self.op)
        with open(evidence, "w", encoding="utf-8") as f:
            json.dump({
                "op": self.op, "value": value,
                "media_sha256": "sha256:" + sha256_file(self.media_path),
                "method": "fake-derived-from-media-sha",
            }, f, ensure_ascii=False, indent=2)
        return {
            "value": value,
            "evidence_uri": evidence,
            "model_versions": {"model": "fake-sha@1"},
        }


class SyncNet:
    """SyncNet 真实后端（V100 实测候选，doctor 判定可行）。

    逐帧人脸检测裁嘴部区 + SyncNet 音视频嵌入 → 全片 LSE-C/LSE-D；
    一次前向同时得两指标（两 op 各自调用独立执行，结果一致性由确定性保证）。
    """

    WEIGHTS = "checkpoints/syncnet_v2.model"

    def __call__(self, media_path, op, seed, workdir):
        import numpy as np  # 镜像内依赖（报批见 PR）；缺失时 ImportError → RUNTIME_ERROR
        import torch

        torch.manual_seed(seed if isinstance(seed, int) else 0)
        torch.cuda.reset_peak_memory_stats()
        start_ev, end_ev = torch.cuda.Event(True), torch.cuda.Event(True)
        start_ev.record()
        av_offset, av_confidence = syncnet_infer(media_path, workdir)
        end_ev.record()
        torch.cuda.synchronize()
        lse_c = float(av_confidence)
        lse_d = float(av_offset)
        value = lse_c if op == "lipsync_lse_c" else lse_d
        evidence = os.path.join(workdir, "evidence_%s.json" % op)
        with open(evidence, "w", encoding="utf-8") as f:
            json.dump({
                "op": op, "value": value, "lse_c": lse_c, "lse_d": lse_d,
                "media_sha256": "sha256:" + sha256_file(media_path),
                "method": "syncnet_v2",
            }, f, ensure_ascii=False, indent=2)
        return {
            "value": value,
            "evidence_uri": evidence,
            "metrics": {
                "gpu_seconds": start_ev.elapsed_time(end_ev) / 1000.0,
                "peak_mem_mb": torch.cuda.max_memory_allocated() / (1024 * 1024),
            },
            "model_versions": {
                "model": "syncnet_v2@1",
                "torch": torch.__version__,
                "numpy": np.__version__,
                "determinism": "seed=%s, manual_seed；纯前向无采样（确定性推理）" % seed,
            },
        }


def syncnet_infer(media_path, workdir):
    """镜像内 SyncNet 推行（run_pipeline 入口改写为库调用）。"""
    from syncnet import inference as sn  # 镜像内 vendored（BSD-3）

    return sn.run_single(media_path, weights_path=SyncNet.WEIGHTS,
                         tmp_dir=os.path.join(workdir, "sn_tmp"))


REGISTRY = {
    "fake": FakeBackend,
    "syncnet": SyncNet(),
}
