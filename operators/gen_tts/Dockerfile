# gen_tts 算子镜像（nvidia runtime 骨架，IR-0007 AC-7 前置 / DECISION-4）
# 依赖报批：CosyVoice2（Apache-2.0）/ torch（BSD-3）——PR 附报批单；
# fake 后端仅用 ffmpeg（apt）与标准库，无 Python 第三方依赖。
FROM pytorch/pytorch:2.4.1-cuda12.1-cudnn9-runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg \
    && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir "torchaudio==2.4.1" \
    "transformers==4.44.2" "accelerate==0.34.2" \
    "git+https://github.com/FunAudioLLM/CosyVoice.git@v0.5.2"

WORKDIR /app
COPY main.py backends.py ./

# nvidia runtime：docker run --gpus all -v <workdir>:<workdir> …
ENTRYPOINT ["python", "main.py"]
CMD ["gen_tts", "--contract-version", "1"]
