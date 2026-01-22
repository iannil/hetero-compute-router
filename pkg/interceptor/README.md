# HCS VRAM Interceptor

GPU memory quota enforcement library using LD_PRELOAD API interception.

## Overview

`libhcs_interceptor.so` intercepts CUDA memory allocation APIs to enforce VRAM quotas at the container level. This enables running multiple GPU workloads on a single GPU with memory isolation, without hardware virtualization support (MIG/MPS).

## Features

- **CUDA API Interception**: cudaMalloc, cudaFree, cudaMemGetInfo, cudaMallocManaged
- **Quota Enforcement**: Reject allocations that would exceed the configured quota
- **Memory Virtualization**: Report virtualized memory info to applications
- **Zero Code Changes**: Works via LD_PRELOAD, no application modifications required

## Build

```bash
# Build the shared library
make

# Build with debug symbols
make CFLAGS="-fPIC -O0 -g -Wall"

# Run mock tests (no CUDA required)
make test-mock

# Run tests with real CUDA
make test
```

## Usage

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `HCS_VRAM_QUOTA` | VRAM quota limit | `16Gi`, `4G`, `1024Mi` |
| `HCS_LOG_LEVEL` | Log verbosity | `debug`, `info`, `warn`, `error` |

### Running Applications

```bash
# Direct usage
LD_PRELOAD=/path/to/libhcs_interceptor.so \
  HCS_VRAM_QUOTA=16Gi \
  python train.py

# In Kubernetes (injected by Webhook)
# The webhook automatically sets:
#   - LD_PRELOAD=/usr/local/hcs/lib/libhcs_interceptor.so
#   - HCS_VRAM_QUOTA=<from pod spec>
```

## Size Format

Supports human-readable size formats:

| Suffix | Multiplier |
|--------|------------|
| `Gi`, `GiB` | 1024³ |
| `G`, `GB` | 1000³ |
| `Mi`, `MiB` | 1024² |
| `M`, `MB` | 1000² |
| `Ki`, `KiB` | 1024 |
| `K`, `KB` | 1000 |

## Intercepted APIs

### NVIDIA CUDA (Priority P0)

- `cudaMalloc(void **devPtr, size_t size)`
- `cudaFree(void *devPtr)`
- `cudaMemGetInfo(size_t *free, size_t *total)`
- `cudaMallocManaged(void **devPtr, size_t size, unsigned int flags)`

### Huawei ACL (Priority P1 - TODO)

- `aclrtMalloc`
- `aclrtFree`
- `aclrtGetMemInfo`

### Hygon HIP (Priority P2 - TODO)

- `hipMalloc`
- `hipFree`
- `hipMemGetInfo`

## How It Works

1. **Initialization**: On library load, read `HCS_VRAM_QUOTA` and initialize quota tracking
2. **Interception**: Use RTLD_NEXT to forward calls to real CUDA functions
3. **Quota Check**: Before each allocation, verify quota won't be exceeded
4. **Tracking**: Maintain a hash table of (ptr → size) for accurate accounting
5. **Virtualization**: `cudaMemGetInfo` returns quota-based values instead of physical memory

## Limitations

- **macOS**: Uses `DYLD_INSERT_LIBRARIES` instead of `LD_PRELOAD`
- **Max Allocations**: Tracks up to 65536 concurrent allocations
- **Thread Safety**: Uses mutex locks (future: atomic operations for better performance)

## Integration with HCS

This library is injected by the HCS Admission Webhook when pods request `ai.compute/vram` resources:

```yaml
resources:
  requests:
    ai.compute/vram: "16Gi"
```

The webhook automatically:
1. Sets `LD_PRELOAD` to the interceptor library
2. Sets `HCS_VRAM_QUOTA` to the requested VRAM
3. Mounts the library from the host

## License

Apache-2.0
