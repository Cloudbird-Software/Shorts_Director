# syncnet_metric 算子镜像（nvidia runtime 骨架，IR-0007 DECISION-4 / 卡 #119）
# 依赖报批：SyncNet v2（BSD-3）/ torch（BSD-3）/ opencv（Apache-2.0）；
# fake 后端仅用标准库，无 Python 第三方依赖。
FROM pytorch/pytorch:2.4.1-cuda12.1-cudnn9-runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg \
    && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir "numpy==1.26.4" "opencv-python-headless==4.10.0.84" \
    "librosa==0.10.2" "scenedetect==0.6.4"
# SyncNet 权重由运维挂载（许可允许再分发亦可 COPY）：-v <weights>:checkpoints/…

WORKDIR /app
COPY main.py backends.py ./

# nvidia runtime：docker run --gpus all -v <workdir>:<workdir> …
ENTRYPOINT ["python", "main.py"]
CMD ["lipsync_lse_c", "--contract-version", "1"]
