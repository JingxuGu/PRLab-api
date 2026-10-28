import errno
import os
from pathlib import Path

try:
    fd = os.open('/dev/nvidia0', os.O_RDWR)
except OSError as exc:
    assert exc.errno in (errno.EPERM, errno.EACCES), exc
else:
    os.close(fd)
    raise AssertionError('GPU device is accessible without an allocation')

cpus = sorted(os.sched_getaffinity(0))
assert 1 <= len(cpus) <= 2, cpus
rel = Path('/proc/self/cgroup').read_text().strip().split('::', 1)[1]
leaf = Path('/sys/fs/cgroup') / rel.lstrip('/')
limits = []
for parent in [leaf, *leaf.parents]:
    if str(parent) == '/sys/fs':
        break
    p = parent / 'memory.max'
    if p.exists() and p.read_text().strip() != 'max':
        limits.append(int(p.read_text().strip()))
assert 256 * 1024**2 in limits, limits
print('PASS GPU denied without GRES')
print('PASS CPU affinity:', cpus)
print('PASS memory limit: 256 MiB')
print('CGROUP:', rel)
