"""vlm_boolean 模型后端注册表——可插拔（换模型不改 C2 契约）。

fake 后端零依赖：answer_hint 透传（联调替身）；无 hint 时按
sha256(媒体内容‖问题) 首字节奇偶确定性作答——无语义的负对照基线
（裁判校准仪器的 sanity check：一致率应显著低于真实 VLM）。
真实后端：Qwen2.5-VL-7B-Instruct（Apache-2.0，doctor 候选清单对齐）。
"""

import hashlib


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.digest()


class FakeBackend:
    """确定性假后端：hint 透传；无 hint 按内容哈希奇偶（负对照）。"""

    def __init__(self, media_path, question, hint, seed, params, workdir):
        self.media_path, self.question, self.hint = media_path, question, hint
        self.params, self.workdir = params, workdir

    def judge(self):
        if self.hint is not None:
            return {
                "answer": self.hint,
                "evidence": "fake 透传 answer_hint（无语义联调替身）",
                "model_versions": {"model": "fake-hint@1"},
            }
        digest = hashlib.sha256(
            sha256_file(self.media_path) + self.question.encode("utf-8")).digest()
        answer = digest[0] % 2 == 0
        return {
            "answer": answer,
            "evidence": "fake 无语义负对照：sha256(媒体内容‖问题) 首字节 "
                        "0x%02x → %s" % (digest[0], answer),
            "model_versions": {"model": "fake-hash@1"},
        }


class QwenVL:
    """Qwen2.5-VL 真实后端（V100 实测候选，doctor 判定可行）。

    贪心解码（do_sample=False）+ torch.manual_seed——同输入同输出；
    非确定性来源显式记入 model_versions.determinism。
    """

    MODEL_ID = "Qwen/Qwen2.5-VL-7B-Instruct"

    def __call__(self, media_path, question, hint, seed, params, workdir):
        import torch  # 镜像内依赖（报批见 PR）；缺失时 ImportError → RUNTIME_ERROR
        from transformers import AutoProcessor, Qwen2_5_VLForConditionalGeneration
        from qwen_vl_utils import process_vision_info

        torch.manual_seed(seed if isinstance(seed, int) else 0)
        model = Qwen2_5_VLForConditionalGeneration.from_pretrained(
            self.MODEL_ID, torch_dtype=torch.float16,
            device_map="auto")
        processor = AutoProcessor.from_pretrained(self.MODEL_ID)
        media = media_path if media_path.lower().endswith(
            (".mp4", ".mov", ".mkv", ".webm", ".avi")) else None
        content = [{
            "type": "image" if media is None else "video",
            **({"image": media_path} if media is None else {"video": media_path}),
        }, {"type": "text", "text": question +
            "\n请先回答“是”或“否”，再用一句话说明依据。"}]
        prompt = processor.apply_chat_template(
            [{"role": "user", "content": content}], tokenize=False,
            add_generation_prompt=True)
        image_inputs, video_inputs = process_vision_info(
            [{"role": "user", "content": content}])
        inputs = processor(text=[prompt], images=image_inputs,
                           videos=video_inputs, padding=True, return_tensors="pt")
        inputs = inputs.to(model.device)
        torch.cuda.reset_peak_memory_stats()
        start_ev, end_ev = torch.cuda.Event(True), torch.cuda.Event(True)
        start_ev.record()
        generated = model.generate(
            **inputs, max_new_tokens=int(params.get("max_new_tokens", 128)),
            do_sample=False)
        end_ev.record()
        torch.cuda.synchronize()
        trimmed = [out[len(inp):] for inp, out in zip(inputs.input_ids, generated)]
        text = processor.batch_decode(
            trimmed, skip_special_tokens=True)[0].strip()
        del model

        head = text.split("\n", 1)[0].strip()
        affirmative = any(w in head for w in ("是", "yes", "Yes", "YES", "true", "True"))
        negative = any(w in head for w in ("否", "no", "No", "NO", "false", "False"))
        if affirmative == negative:  # 双真=含糊，双假=未识别
            raise ValueError("无法从回答首行解析布尔判定: %r" % head)
        return {
            "answer": affirmative,
            "evidence": text,
            "metrics": {
                "gpu_seconds": start_ev.elapsed_time(end_ev) / 1000.0,
                "peak_mem_mb": torch.cuda.max_memory_allocated() / (1024 * 1024),
            },
            "model_versions": {
                "model": "qwen2.5-vl-7b-instruct@1",
                "torch": torch.__version__,
                "determinism": "seed=%s, manual_seed；贪心解码 do_sample=False" % seed,
                "raw_head": head,
            },
        }


REGISTRY = {
    "fake": FakeBackend,
    "qwen-vl": QwenVL(),
}
