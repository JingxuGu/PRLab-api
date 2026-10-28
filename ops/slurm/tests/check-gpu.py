import ctypes as C
import os
import time
from pathlib import Path

lib = C.CDLL('libcuda.so.1')
lib.cuInit.argtypes = [C.c_uint]
lib.cuDeviceGetCount.argtypes = [C.POINTER(C.c_int)]
lib.cuDevicePrimaryCtxRetain.argtypes = [C.POINTER(C.c_void_p), C.c_int]
lib.cuCtxSetCurrent.argtypes = [C.c_void_p]
lib.cuMemAlloc_v2.argtypes = [C.POINTER(C.c_uint64), C.c_size_t]
lib.cuMemFree_v2.argtypes = [C.c_uint64]
lib.cuDevicePrimaryCtxRelease_v2.argtypes = [C.c_int]

def call(name, *args):
    status = getattr(lib, name)(*args)
    if status != 0:
        raise RuntimeError(f'{name}: CUDA error {status}')

call('cuInit', 0)
count = C.c_int()
call('cuDeviceGetCount', C.byref(count))
assert count.value == 1, count.value
ctx = C.c_void_p()
call('cuDevicePrimaryCtxRetain', C.byref(ctx), 0)
try:
    call('cuCtxSetCurrent', ctx)
    ptr = C.c_uint64()
    call('cuMemAlloc_v2', C.byref(ptr), 1024**2)
    try:
        print('PASS CUDA context and 1 MiB allocation; no compute kernel launched', flush=True)
        print('CUDA_VISIBLE_DEVICES:', os.environ.get('CUDA_VISIBLE_DEVICES'), flush=True)
        marker = os.environ.get('GPU_TEST_MARKER')
        if marker:
            Path(marker).write_text(str(os.getpid()))
        time.sleep(float(os.environ.get('GPU_TEST_HOLD_SECONDS', '0')))
    finally:
        call('cuMemFree_v2', ptr)
finally:
    call('cuDevicePrimaryCtxRelease_v2', 0)
