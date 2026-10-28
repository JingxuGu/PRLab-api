#!/usr/bin/env bash
set -euo pipefail
umask 022
cd /home/llmops/slurm-setup/config
test ! -e /etc/slurm/slurm.conf
test ! -e /etc/systemd/system/slurmctld.service
test ! -e /etc/systemd/system/slurmd.service
if ! id slurm >/dev/null 2>&1; then
    sudo -n useradd --system --user-group --home-dir /var/lib/slurm --no-create-home --shell /usr/sbin/nologin slurm
fi
sudo -n install -d -o root -g root -m 0755 /etc/slurm /var/lib/slurm
sudo -n install -d -o slurm -g slurm -m 0700 /var/lib/slurm/slurmctld
sudo -n install -d -o root -g root -m 0700 /var/lib/slurm/slurmd
sudo -n install -d -o slurm -g slurm -m 0750 /var/log/slurm
sudo -n install -o root -g root -m 0644 slurm.conf cgroup.conf gres.conf /etc/slurm/
sudo -n install -o root -g root -m 0755 gpu-idle-prolog.sh /etc/slurm/
sudo -n install -o root -g root -m 0644 slurmctld.service slurmd.service /etc/systemd/system/
if ! sudo -n test -s /etc/munge/munge.key; then
    sudo -n sh -c 'umask 077; dd if=/dev/urandom of=/etc/munge/munge.key bs=1024 count=1 status=none'
    sudo -n chown munge:munge /etc/munge/munge.key
    sudo -n chmod 0400 /etc/munge/munge.key
fi
for binary in /opt/slurm/26.05.5/bin/*; do
    target=/usr/local/bin/$(basename "$binary")
    if test -e "$target" || test -L "$target"; then
        echo "Existing command path: $target" >&2
        exit 1
    fi
    sudo -n ln -s "$binary" "$target"
done
sudo -n tee /etc/logrotate.d/slurm-llmops >/dev/null <<'ROTATE'
/var/log/slurm/*.log {
    weekly
    rotate 8
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
    su slurm slurm
}
ROTATE
sudo -n systemd-analyze verify /etc/systemd/system/slurmctld.service /etc/systemd/system/slurmd.service
sudo -n systemctl daemon-reload
sudo -n systemctl enable --now munge
munge -n | unmunge | head -n 3
sudo -n systemctl enable --now slurmctld slurmd
sleep 3
systemctl is-active munge slurmctld slurmd
/opt/slurm/26.05.5/bin/scontrol ping
/opt/slurm/26.05.5/bin/sinfo -Nel
