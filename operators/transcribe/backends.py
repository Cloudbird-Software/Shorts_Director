"""transcribe 模型后端注册表——可插拔（换模型不改 C2 契约）。

fake 后端零依赖：返回 text_hint（上游口播源文案的确定性透传——
fake TTS 音频无语义，转写链路的联调替身；三要素真实验证发生在
V100 实测：CosyVoice 音频 + whisper 独立转写）。
真实后端：faster-whisper（MIT）。
"""


class FakeBackend:
    """确定性假后端：透传 text_hint（无语义音频的转写联调替身）。"""

    def __init__(self, audio_path, hint, seed, params, workdir):
        self.audio_path, self.hint = audio_path, hint
        self.params, self.workdir = params, workdir

    def transcribe(self):
        if not isinstance(self.hint, str) or not self.hint.strip():
            # fail-closed：fake 无透传锚即无法产出转写文本
            raise ValueError(
                "fake 后端需要 inputs.text_hint（fake 音频无语义；"
                "真实链路用 whisper 从音频独立转写）")
        return {
            "text": self.hint,
            "model_versions": {"model": "fake-hint@1"},
        }


class WhisperSTT:
    """faster-whisper 真实后端（V100 实测候选，doctor 判定可行）。

    忽略 text_hint——从音频独立转写（三要素验证不自我循环）。
    """

    MODEL_ID = "small"

    def __call__(self, audio_path, hint, seed, params, workdir):
        import torch  # 镜像内依赖（报批见 PR）；缺失时 ImportError → RUNTIME_ERROR
        from faster_whisper import WhisperModel

        torch.manual_seed(seed if isinstance(seed, int) else 0)
        model = WhisperModel(
            params.get("whisper_model", self.MODEL_ID),
            device="cuda" if torch.cuda.is_available() else "cpu",
            compute_type="float16")
        torch.cuda.reset_peak_memory_stats()
        start_ev, end_ev = torch.cuda.Event(True), torch.cuda.Event(True)
        start_ev.record()
        segments, info = model.transcribe(
            audio_path, language=params.get("language") or None)
        text = "".join(s.text for s in segments).strip()
        end_ev.record()
        torch.cuda.synchronize()
        del model
        return {
            "text": text,
            "metrics": {
                "gpu_seconds": start_ev.elapsed_time(end_ev) / 1000.0,
                "peak_mem_mb": torch.cuda.max_memory_allocated() / (1024 * 1024),
            },
            "model_versions": {
                "model": "faster-whisper/%s@1" % params.get("whisper_model", self.MODEL_ID),
                "torch": torch.__version__,
                "determinism": "seed=%s, manual_seed；纯推理（beam 无采样）" % seed,
                "language": info.language,
            },
        }


REGISTRY = {
    "fake": FakeBackend,
    "whisper": WhisperSTT(),
}
