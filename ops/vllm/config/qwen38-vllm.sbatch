#!/bin/bash
#SBATCH --job-name=qwen38-vllm
#SBATCH --partition=llm
#SBATCH --gres=gpu:rtx_5090:1
#SBATCH --nodes=1
#SBATCH --ntasks=1
#SBATCH --cpus-per-task=8
#SBATCH --mem=28000M
#SBATCH --time=1-00:00:00
#SBATCH --requeue
#SBATCH --open-mode=append
#SBATCH --output=/home/llmops/llm/logs/vllm-%j.log

set -euo pipefail
umask 077
cd /home/llmops/llm
test -s config/model-verified.json
export HF_HOME=/home/llmops/llm/cache/huggingface
export HF_HUB_OFFLINE=1
export TRANSFORMERS_OFFLINE=1
export HF_HUB_DISABLE_TELEMETRY=1
export VLLM_NO_USAGE_STATS=1
export DO_NOT_TRACK=1
export OMP_NUM_THREADS=2
export MKL_NUM_THREADS=2
export TOKENIZERS_PARALLELISM=false
export MAX_JOBS=2
export CUDA_CACHE_PATH=/home/llmops/llm/cache/cuda
export TRITON_CACHE_DIR=/home/llmops/llm/cache/triton
export FLASHINFER_WORKSPACE_BASE=/home/llmops/llm/cache/flashinfer
export VLLM_CACHE_ROOT=/home/llmops/llm/cache/vllm
export VLLM_API_KEY
VLLM_API_KEY=$(cat /home/llmops/llm/config/backend-key)
printf 'Starting Qwen3.8 NVFP4: job=%s restart=%s GPU=%s\n' "$SLURM_JOB_ID" "${SLURM_RESTART_COUNT:-0}" "${CUDA_VISIBLE_DEVICES:-unset}"
exec /home/llmops/llm/venv/bin/vllm serve /home/llmops/llm/models/Qwen3.8-27B-NVFP4 \
    --served-model-name qwen3.8-27b \
    --host 127.0.0.1 --port 8001 \
    --tensor-parallel-size 1 \
    --max-model-len 8192 \
    --max-num-seqs 4 \
    --max-num-batched-tokens 8192 \
    --gpu-memory-utilization 0.90 \
    --kv-cache-dtype auto \
    --enforce-eager \
    --limit-mm-per-prompt '{"image":1,"video":0}' \
    --mm-processor-kwargs '{"size":{"longest_edge":1048576,"shortest_edge":65536}}' \
    --reasoning-parser qwen3 \
    --enable-auto-tool-choice --tool-call-parser qwen3_coder
