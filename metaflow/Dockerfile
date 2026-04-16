FROM pytorch/pytorch:2.8.0-cuda12.9-cudnn9-runtime

RUN pip install --no-cache-dir --break-system-packages \
    metaflow==2.19.16 \
    awscli \
    boto3 \
    datasets==4.3.0 \
    transformers==4.57.1 \
    tensorboard==2.20.0 \
    matplotlib==3.10.8 \
    torch==2.11.0 \
    peft==0.19.1 \
    "unsloth[colab-new] @ git+https://github.com/unslothai/unsloth.git" \
    "bitsandbytes>=0.43.0"

RUN apt-get update && apt-get install -y \
    build-essential \
    gcc \
    g++ \
    && rm -rf /var/lib/apt/lists/*

ENV CC=gcc
ENV LD_LIBRARY_PATH=/usr/local/cuda/lib64:$LD_LIBRARY_PATH
    
RUN ln -sf /usr/bin/python3 /usr/bin/python


RUN mkdir /logs && chown 1000 /logs
RUN mkdir /metaflow && chown 1000 /metaflow