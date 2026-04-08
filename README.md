# NN-HIGH-PERFORMANCE: Deep Learning Optimization & MLOps

A comprehensive engineering project focused on **High-Performance Computing (HPC)** and **MLOps** practices applied to the fine-tuning of the **DeepSeek-V2 Lite** language model. This repository demonstrates a standardized workflow from a local development environment (Minikube) to a production-ready cloud architecture on AWS using Infrastructure as Code (IaC).

## 🚀 Project Overview

The core objective is to explore cutting-edge optimization techniques that enable efficient training of Large Language Models (LLMs) on both consumer-grade and cloud-based hardware.

### Key Components
* **Model:** DeepSeek-V2 Lite (MoE architecture).
* **Orchestration:** [Metaflow](https://metaflow.org/) (DAG-based workflows).
* **Infrastructure:** AWS (EC2 g5.2xlarge with NVIDIA A10G GPUs).
* **Provisioning:** Infrastructure as Code (CloudFormation).
* **Local Dev:** Minikube/Kubernetes for local pipeline validation.

---

## 🛠️ Optimization & HPC Techniques

The project explores several layers of performance optimization to reduce VRAM footprint and accelerate convergence:

### 1. Quantization Strategies
* **Training Aware Quantization (TAQ):** Optimizing precision during the training loop.
* **Post Training Quantization (PTQ):** Model compression after convergence for efficient inference.
* **Mixed Precision (BF16):** Utilizing NVIDIA Tensor Cores to balance numerical stability and throughput.

---

## 📊 Results & Evaluation

The experiment successfully validated the portability of Metaflow DAGs between environments.

* **Training Stability:** Despite the "noisy" loss curve (inherent to small batch sizes and diverse datasets), the model showed consistent downward convergence.
* **Environment Parity:** Validated that local Minikube workflows can be replicated on AWS `@batch` decorators with zero logic changes.

---

## 📋 Requirements

* **Python 3.10+**
* **Docker** (for local execution)
* **AWS CLI** (configured for remote deployment)
* **NVIDIA Drivers & CUDA Toolkit** (for GPU-accelerated steps)
---

**Developed by [Alejo Vasquero](https://github.com/alejovasquero)**
