FROM pytorch/pytorch:2.8.0-cuda12.9-cudnn9-runtime

RUN pip install --no-cache-dir --break-system-packages \
    metaflow==2.19.16 \
    awscli \
    boto3 \
    click \
    requests

RUN apt-get update && apt-get install -y \
    build-essential \
    gcc \
    g++ \
    && rm -rf /var/lib/apt/lists/*

ENV CC=gcc
    
RUN ln -sf /usr/bin/python3 /usr/bin/python


RUN mkdir /logs && chown 1000 /logs
RUN mkdir /metaflow && chown 1000 /metaflow
ENV HOME=/metaflow
WORKDIR /metaflow