#!/usr/bin/env bash
set -euo pipefail
umask 022
cd /home/llmops/slurm-setup/source
printf '%s  %s\n' 4680cafc82b0689865c885eb703441e7ebf764896c40fd03a50f2fcb60664f68 slurm-26.05.5.tar.bz2 | sha256sum -c -
printf '%s  %s\n' ff7d309fdff7ceddfe15c1e79eaff25f3126a134f29f44d4e85571f187a6bab8 hwloc-2.12.2.tar.gz | sha256sum -c -
tar -xzf hwloc-2.12.2.tar.gz
cd hwloc-2.12.2
./configure --prefix=/opt/slurm-deps/hwloc-2.12.2 --disable-io --disable-libxml2 --disable-cairo > /home/llmops/slurm-setup/logs/hwloc-configure.log 2>&1
make -j2 > /home/llmops/slurm-setup/logs/hwloc-build.log 2>&1
sudo -n make install > /home/llmops/slurm-setup/logs/hwloc-install.log 2>&1
printf 'HWLOC_INSTALLED\n'
cd /home/llmops/slurm-setup/source
tar -xjf slurm-26.05.5.tar.bz2
cd slurm-26.05.5
./configure --prefix=/opt/slurm/26.05.5 --sysconfdir=/etc/slurm --with-munge=/usr --with-hwloc=/opt/slurm-deps/hwloc-2.12.2 --disable-dependency-tracking > /home/llmops/slurm-setup/logs/slurm-configure.log 2>&1
make -j2 > /home/llmops/slurm-setup/logs/slurm-build.log 2>&1
sudo -n make install > /home/llmops/slurm-setup/logs/slurm-install.log 2>&1
test -f /opt/slurm/26.05.5/lib/slurm/cgroup_v2.so
test -f /opt/slurm/26.05.5/lib/slurm/task_cgroup.so
test -f /opt/slurm/26.05.5/lib/slurm/auth_munge.so
/opt/slurm/26.05.5/sbin/slurmd -V
/opt/slurm/26.05.5/sbin/slurmd -C
printf 'SLURM_INSTALLED\n'
