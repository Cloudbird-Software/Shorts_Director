"""gen_lipsync 模型后端注册表——可插拔（换模型不改 C2 契约）。

候选与 internal/doctor candidates.go 对齐：wav2lip（学术研究用途许可，
本仓仅做可行性实测评估，不商用发布产物——PR 报批单说明）。
fake 后端零依赖（ffmpeg 静态图+音轨），供无 GPU 的管道联调。
"""

import hashlib
import os
import shutil
import subprocess


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return "sha256:" + h.hexdigest()


def _ffmpeg_available():
    return shutil.which("ffmpeg") is not None


class FakeBackend:
    """确定性假后端：静态人像图循环 + 音轨 → mp4（-shortest 对齐音频时长）。

    不做口型生成（无语义内容），仅供无 GPU 环境的端到端联调；
    口型质量判定交给 syncnet_metric 探针（fake 派生值恒过阈值）。
    """

    def __init__(self, image_path, audio_path, fps, seed, params, workdir):
        self.image_path, self.audio_path, self.fps = image_path, audio_path, fps
        self.seed = seed if isinstance(seed, int) else 0
        self.params, self.workdir = params, workdir

    def generate(self):
        if not _ffmpeg_available():
            raise RuntimeError("fake 后端导出需要 ffmpeg")
        w = int(self.params.get("width", 540))
        h = int(self.params.get("height", 960))
        out = os.path.join(self.workdir, "out_fake_lipsync.mp4")
        subprocess.run(
            ["ffmpeg", "-y", "-loglevel", "error",
             "-loop", "1", "-framerate", str(self.fps), "-i", self.image_path,
             "-i", self.audio_path,
             "-vf", "scale=%d:%d:force_original_aspect_ratio=increase,"
                    "crop=%d:%d" % (w, h, w, h),
             "-c:v", "libx264", "-preset", "slow", "-tune", "stillimage",
             "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", out],
            check=True)
        return {
            "video_path": out,
            "content_hash": sha256_file(out),
            "model_versions": {"model": "fake-loop@1", "ffmpeg": "lavfi-loop"},
        }


class Wav2Lip:
    """wav2lip 真实后端（V100 实测候选，doctor 判定可行）。

    人脸检测（batch内 S3FD）+ wav2lip 生成器逐帧口型合成；
    输出仍以音频轨为时长锚。确定性：torch.manual_seed。
    """

    WEIGHTS = "checkpoints/wav2lip_gan.pth"

    def __call__(self, image_path, audio_path, fps, seed, params, workdir):
        import torch  # 镜像内依赖（报批见 PR）；缺失时 ImportError → RUNTIME_ERROR

        torch.manual_seed(seed)
        torch.cuda.reset_peak_memory_stats()
        start_ev, end_ev = torch.cuda.Event(True), torch.cuda.Event(True)
        start_ev.record()
        frames = wav2lip_infer(image_path, audio_path, fps, params)
        end_ev.record()
        torch.cuda.synchronize()
        out = export_mp4(frames, audio_path, fps, workdir)
        return {
            "video_path": out,
            "content_hash": sha256_file(out),
            "metrics": {
                "gpu_seconds": start_ev.elapsed_time(end_ev) / 1000.0,
                "peak_mem_mb": torch.cuda.max_memory_allocated() / (1024 * 1024),
            },
            "model_versions": {
                "model": "wav2lip_gan@1",
                "torch": torch.__version__,
                "determinism": "seed=%d, manual_seed; 已知非确定性来源: "
                               "cuDNN/cuBLAS 归约顺序（AC-3 差异显式记录）" % seed,
                "license": "学术研究用途；产物仅实验评估不商用（报批单）",
            },
        }


def wav2lip_infer(image_path, audio_path, fps, params):
    """镜像内的 wav2lip 推理（inference.py 入口改写为库调用）。"""
    from wav2lip import inference as w2l  # CosyVoice 镜像内 vendored

    return w2l.run_single(image_path, audio_path, fps,
                          weights_path=Wav2Lip.WEIGHTS,
                          resize_factor=int(params.get("resize_factor", 1)))


def export_mp4(frames_dir, audio_path, fps, workdir):
    """帧目录 + 音轨 → mp4（确定性：libx264 + -shortest 音频锚）。"""
    out = os.path.join(workdir, "out_lipsync.mp4")
    subprocess.run(
        ["ffmpeg", "-y", "-loglevel", "error",
         "-framerate", str(fps), "-i", os.path.join(frames_dir, "%05d.jpg"),
         "-i", audio_path,
         "-c:v", "libx264", "-preset", "slow", "-tune", "stillimage",
         "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", out],
        check=True)
    return out


REGISTRY = {
    "fake": FakeBackend,
    "wav2lip": Wav2Lip(),
}
