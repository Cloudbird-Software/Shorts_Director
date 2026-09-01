"""gen_tts 模型后端注册表——可插拔（换模型不改 C2 契约）。

候选与 internal/doctor candidates.go 对齐：cosyvoice2-0.5b（Apache-2.0）。
fake 后端零依赖（ffmpeg lavfi 正弦音），供无 GPU 的管道联调。

真实后端（CosyVoice）：默认音色免克隆（卡 #119：不引入声音克隆链路）；
确定性：torch.manual_seed；已知非确定性来源显式记入 model_versions.determinism。
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
    """确定性假后端：ffmpeg lavfi 正弦音，seed 决定基频，文案长度决定时长。

    用于无 GPU 环境的端到端联调，不产生语义内容；时长估算
    duration = max(1.0, 字符数 * 0.24 / speed)——只依赖 text 与 speed，
    与 seed 一起保证同输入同输出。
    """

    def __init__(self, text, seed, params, workdir):
        self.text = text
        self.seed = seed if isinstance(seed, int) else 0
        self.params, self.workdir = params, workdir

    def generate(self):
        if not _ffmpeg_available():
            raise RuntimeError("fake 后端导出需要 ffmpeg")
        speed = float(self.params.get("speed", 1.0))
        if speed <= 0:
            speed = 1.0
        sample_rate = int(self.params.get("sample_rate", 24000))
        freq = 180 + (self.seed % 7) * 60  # 180–540 Hz，seed 决定
        duration = max(1.0, round(len(self.text) * 0.24 / speed, 3))
        out = os.path.join(self.workdir, "out_fake.wav")
        subprocess.run(
            ["ffmpeg", "-y", "-loglevel", "error",
             "-f", "lavfi", "-i",
             "sine=frequency=%d:sample_rate=%d:duration=%s" % (
                 freq, sample_rate, duration),
             "-c:a", "pcm_s16le", out],
            check=True)
        return {
            "audio_path": out,
            "content_hash": sha256_file(out),
            "duration_sec": duration,
            "model_versions": {"model": "fake-lavfi@1", "ffmpeg": "lavfi"},
        }


class CosyVoiceTTS:
    """CosyVoice 2 真实后端（V100 实测候选，doctor 判定可行）。

    默认音色免克隆：voice 留空时用模型自带音色（如中文女声），
    不加载任何说话人参考音频——规避声音克隆的合规面。
    """

    REPO = "FunAudioLLM/CosyVoice2-0.5B"
    DEFAULT_VOICE = "中文女"

    def __call__(self, text, seed, params, workdir):
        import torch  # 镜像内依赖（报批见 PR）；缺失时 ImportError → RUNTIME_ERROR
        from cosyvoice.cli.cosyvoice import CosyVoice2

        voice = params.get("voice") or self.DEFAULT_VOICE
        speed = float(params.get("speed", 1.0))
        torch.manual_seed(seed)
        model = CosyVoice2(self.REPO, load_jit=False, load_trt=False)
        torch.cuda.reset_peak_memory_stats()
        start_ev, end_ev = torch.cuda.Event(True), torch.cuda.Event(True)
        start_ev.record()
        chunks = list(model.inference_sft(
            text, voice, speed=speed, stream=False))
        end_ev.record()
        torch.cuda.synchronize()
        out = os.path.join(workdir, "out_tts.wav")
        torchaudio_save(chunks, out)
        return {
            "audio_path": out,
            "content_hash": sha256_file(out),
            "duration_sec": wav_duration(out),
            "metrics": {
                "gpu_seconds": start_ev.elapsed_time(end_ev) / 1000.0,
                "peak_mem_mb": torch.cuda.max_memory_allocated() / (1024 * 1024),
            },
            "model_versions": {
                "model": self.REPO,
                "torch": torch.__version__,
                "determinism": "seed=%d, manual_seed; 已知非确定性来源: "
                               "采样与 cuDNN/cuBLAS 归约顺序（AC-3 差异显式记录）" % seed,
            },
        }


def torchaudio_save(chunks, out):
    import torch
    import torchaudio

    frames = [torch.from_numpy(c["tts_speech"]) for c in chunks]
    torchaudio.save(out, torch.cat(frames, dim=-1), 24000)


def wav_duration(path):
    out = subprocess.run(
        ["ffprobe", "-v", "error", "-show_entries", "format=duration",
         "-of", "default=noprint_wrappers=1:nokey=1", path],
        capture_output=True, text=True, check=True)
    return round(float(out.stdout.strip()), 3)


REGISTRY = {
    "fake": FakeBackend,
    "cosyvoice2-0.5b": CosyVoiceTTS(),
}
