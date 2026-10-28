#!/bin/bash
set -euo pipefail
# This host has one full-GPU GRES. Any compute context before a new GPU
# allocation starts belongs to an external process or to an unclean prior job.
# Refuse admission; never signal any existing compute process.
if [[ -z "${SLURM_JOB_GPUS:-}" ]]; then
    exit 0
fi
if ! gpu_pids=$(/usr/bin/timeout 5 /usr/bin/nvidia-smi --id=0 --query-compute-apps=pid --format=csv,noheader,nounits); then
    /usr/bin/logger -t slurm-gpu-guard 'GPU query failed; refusing new GPU job'
    exit 1
fi
if [[ -n "${gpu_pids//[[:space:]]/}" ]]; then
    /usr/bin/logger -t slurm-gpu-guard "GPU occupied before job ${SLURM_JOB_ID:-unknown}; refusing admission (existing processes untouched)"
    printf 'GPU already has compute contexts; refusing this new job. Administrator must inspect and resume the drained node.\n' >&2
    exit 1
fi
