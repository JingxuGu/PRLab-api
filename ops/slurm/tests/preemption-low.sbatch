#!/bin/bash
printf "START restart=%s\n" "${SLURM_RESTART_COUNT:-0}"
sleep 150
