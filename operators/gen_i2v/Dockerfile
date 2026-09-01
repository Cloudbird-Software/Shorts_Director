# gen_i2v 算子镜像（nvidia runtime 骨架，IR-0007 AC-3 / E1）
# 依赖报批：torch（BSD-3）/ diffusers（Apache-2.0）——PR 附报批单；
# fake 后端仅用 ffmpeg（apt）与标准库，无 Python 第三方依赖。
FROM pytorch/pytorch:2.4.1-cuda12.1-cudnn9-runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg \
    && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir "diffusers==0.31.0" "transformers==4.44.2" \
    "accelerate==0.34.2" "safetensors==0.4.5" "pillow==10.4.0"

WORKDIR /app
COPY main.py backends.py ./

# nvidia runtime：docker run --gpus all -v <workdir>:<workdir> …
ENTRYPOINT ["python", "main.py"]
CMD ["gen_i2v", "--contract-version", "1"]
