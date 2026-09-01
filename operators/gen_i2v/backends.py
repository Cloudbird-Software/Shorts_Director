"""gen_i2v 模型后端注册表——可插拔（换模型不改 C2 契约）。

首批候选与 internal/doctor 候选清单对齐：ltx-video-2b / wan2.1-i2v-1.3b /
cogvideox-5b-i2v（卡面写作 CogVideoX-2b，但 2b 仅 t2v，i2v 候选实为 5b——
与 doctor candidates.go 一致）。fake 后端零依赖，供无 GPU 的管道联调。

真实后端走 diffusers 统一接口；确定性：torch.manual_seed + 独立 generator，
已知非确定性来源（cuDNN/cuBLAS 归约顺序）显式记入 model_versions.determinism
（AC-3 重放条款的"差异来源显式记录"通道）。
"""

import hashlib
import os
import shutil
import subprocess


def _ffmpeg_available():
    return shutil.which("ffmpeg") is not None


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return "sha256:" + h.hexdigest()


def _export_pngs_to_mp4(frames, fps, workdir, tag):
    """frames: PIL Image 列表 → ffmpeg 编码 mp4（确定性：无时间戳抖动）。"""
    if not _ffmpeg_available():
        raise RuntimeError("导出需要 ffmpeg（算子镜像应内含）")
    fdir = os.path.join(workdir, "frames_" + tag)
    os.makedirs(fdir, exist_ok=True)
    for i, img in enumerate(frames):
        img.save(os.path.join(fdir, "%05d.png" % i))
    out = os.path.join(workdir, "out_%s.mp4" % tag)
    subprocess.run(
        ["ffmpeg", "-y", "-loglevel", "error", "-framerate", str(fps),
         "-i", os.path.join(fdir, "%05d.png"),
         "-c:v", "libx264", "-preset", "slow", "-tune", "stillimage",
         "-pix_fmt", "yuv420p", out],
        check=True)
    return out


class FakeBackend:
    """确定性假后端：ffmpeg lavfi 合成纯色视频，seed 决定颜色。

    用于无 GPU 环境的端到端联调（E1 冒烟前的管道验证），不产生语义内容。
    """

    PALETTE = ["0x101418", "0x8B1A1A", "0x1A5276", "0x145A32", "0x4A235A"]

    def __init__(self, image_path, prompt, duration_sec, fps, seed, params, workdir):
        self.duration_sec, self.fps = duration_sec, fps
        self.seed = seed if isinstance(seed, int) else 0
        self.params, self.workdir = params, workdir

    def generate(self):
        if not _ffmpeg_available():
            raise RuntimeError("fake 后端导出需要 ffmpeg")
        color = self.PALETTE[self.seed % len(self.PALETTE)]
        w = int(self.params.get("width", 320))
        h = int(self.params.get("height", 240))
        out = os.path.join(self.workdir, "out_fake.mp4")
        subprocess.run(
            ["ffmpeg", "-y", "-loglevel", "error",
             "-f", "lavfi", "-i", "color=c=%s:s=%dx%d:d=%s:r=%d" % (
                 color, w, h, self.duration_sec, self.fps),
             "-c:v", "libx264", "-preset", "slow", "-tune", "stillimage",
             "-pix_fmt", "yuv420p", out],
            check=True)
        return {
            "video_path": out,
            "content_hash": sha256_file(out),
            "model_versions": {"model": "fake-lavfi@1", "ffmpeg": "lavfi"},
        }


class DiffusersI2V:
    """diffusers 统一接口的真实图生视频后端（V100 实测候选）。"""

    def __init__(self, repo, pipeline, vram_note, num_frames_rule=None):
        self.repo, self.pipeline = repo, pipeline
        self.vram_note = vram_note
        self.num_frames_rule = num_frames_rule or (lambda n: n)

    def __call__(self, image_path, prompt, duration_sec, fps, seed, params, workdir):
        import diffusers  # 镜像内依赖（报批见 PR）；缺失时 ImportError → RUNTIME_ERROR
        import torch
        from PIL import Image

        torch.manual_seed(seed)
        gen = torch.Generator(device="cuda" if torch.cuda.is_available() else "cpu")
        gen = gen.manual_seed(seed)
        num_frames = self.num_frames_rule(int(round(duration_sec * fps)))
        pipe_cls = getattr(diffusers, self.pipeline)
        pipe = pipe_cls.from_pretrained(self.repo, torch_dtype=torch.float16)
        if torch.cuda.is_available():
            pipe = pipe.to("cuda")
            torch.cuda.reset_peak_memory_stats()
        if torch.cuda.is_available():
            start_ev, end_ev = torch.cuda.Event(True), torch.cuda.Event(True)
            start_ev.record()
        frames = pipe(
            image=Image.open(image_path).convert("RGB"),
            prompt=prompt,
            num_frames=num_frames,
            height=int(params.get("height", 480)),
            width=int(params.get("width", 848)),
            num_inference_steps=int(params.get("num_inference_steps", 25)),
            generator=gen,
        ).frames[0]
        if torch.cuda.is_available():
            end_ev.record()
            torch.cuda.synchronize()
        path = _export_pngs_to_mp4(frames, fps, workdir, self.pipeline.lower())
        metrics = {}
        if torch.cuda.is_available():
            metrics = {
                "gpu_seconds": start_ev.elapsed_time(end_ev) / 1000.0,
                "peak_mem_mb": torch.cuda.max_memory_allocated() / (1024 * 1024),
            }
        return {
            "video_path": path,
            "content_hash": sha256_file(path),
            "metrics": metrics,
            "model_versions": {
                "model": self.repo,
                "torch": torch.__version__,
                "determinism": "seed=%d, manual_seed+generator; 已知非确定性来源: "
                               "cuDNN/cuBLAS 归约顺序（AC-3 差异显式记录）" % seed,
                "vram_note": self.vram_note,
            },
        }


def _ltx_frames(n):  # LTX 帧数规则：8k+1
    return max(9, (n // 8) * 8 + 1)


REGISTRY = {
    "fake": FakeBackend,
    "ltx-video-2b": DiffusersI2V(
        "Lightricks/LTX-Video-0.9.5-dev-2b", "LTXImageToVideoPipeline",
        "V100 32G fp16 可行（doctor 判定）", _ltx_frames),
    "wan2.1-i2v-1.3b": DiffusersI2V(
        "Wan-AI/Wan2.1-I2V-1.3B-Diffusers", "WanImageToVideoPipeline",
        "V100 32G fp16 可行（doctor 判定）"),
    "cogvideox-5b-i2v": DiffusersI2V(
        "THUDM/CogVideoX-5b-I2V", "CogVideoXImageToVideoPipeline",
        "V100 32G fp16 可行（doctor 判定）"),
}
