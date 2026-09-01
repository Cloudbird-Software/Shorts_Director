# gen_lipsync 算子镜像（nvidia runtime 骨架，IR-0007 AC-7 前置 / DECISION-4）
# 依赖报批：wav2lip（学术研究用途许可——仅实验评估不商用）/ torch（BSD-3）；
# fake 后端仅用 ffmpeg（apt）与标准库，无 Python 第三方依赖。
FROM pytorch/pytorch:2.4.1-cuda12.1-cudnn9-runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg \
    && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir "numpy==1.26.4" "opencv-python-headless==4.10.0.84" \
    "librosa==0.10.2" "tqdm==4.66.5"
# wav2lip 权重由运维挂载（学术许可不进镜像分发）：-v <weights>:checkpoints/…

WORKDIR /app
COPY main.py backends.py ./

# nvidia runtime：docker run --gpus all -v <workdir>:<workdir> …
ENTRYPOINT ["python", "main.py"]
CMD ["gen_lipsync", "--contract-version", "1"]
