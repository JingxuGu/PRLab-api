#!/bin/bash
#SBATCH --job-name=research-gpu
#SBATCH --partition=research
#SBATCH --gres=gpu:rtx_5090:1
#SBATCH --ntasks=1
#SBATCH --cpus-per-task=8
#SBATCH --mem=24G
#SBATCH --time=04:00:00
#SBATCH --output=research-%j.log

set -euo pipefail
if [[ $# -eq 0 ]]; then
    printf 'Usage: sbatch research-gpu.sbatch python train.py [arguments...]\n' >&2
    exit 64
fi
cd "$SLURM_SUBMIT_DIR"
exec srun "$@"
