# vlm_boolean 算子镜像（nvidia runtime 骨架，IR-0007 AC-8 / BEH-7）
# 依赖报批：transformers（Apache-2.0）/ qwen-vl-utils（Apache-2.0）/ torch（BSD-3）
# ——PR 附报批单；fake 后端仅用标准库，无 Python 第三方依赖。
FROM pytorch/pytorch:2.4.1-cuda12.1-cudnn9-runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg \
    && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir \
    "transformers==4.49.0" "qwen-vl-utils==0.0.10" "accelerate==1.3.0"

WORKDIR /app
COPY main.py backends.py ./

# nvidia runtime：docker run --gpus all -v <workdir>:<workdir> …
ENTRYPOINT ["python", "main.py"]
CMD ["vlm_boolean", "--contract-version", "1"]
