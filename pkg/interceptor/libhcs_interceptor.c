/*
 * HCS VRAM Interceptor Library
 *
 * This library intercepts GPU memory allocation APIs to enforce VRAM quotas.
 * It uses LD_PRELOAD to hook memory allocation functions and track usage.
 *
 * Supported APIs:
 *   - NVIDIA CUDA: cudaMalloc, cudaFree, cudaMemGetInfo, cudaMallocManaged
 *   - Huawei ACL:  aclrtMalloc, aclrtFree, aclrtGetMemInfo
 *   - AMD/Hygon HIP: hipMalloc, hipFree, hipMemGetInfo
 *
 * Environment Variables:
 *   HCS_VRAM_QUOTA  - VRAM quota in bytes or human-readable format (e.g., "16Gi")
 *   HCS_LOG_LEVEL   - Log level: debug, info, warn, error (default: warn)
 *
 * Usage:
 *   LD_PRELOAD=/path/to/libhcs_interceptor.so HCS_VRAM_QUOTA=16Gi ./your_app
 *
 * Copyright (c) 2024 HCS Project
 * SPDX-License-Identifier: Apache-2.0
 */

#define _GNU_SOURCE
#include <dlfcn.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdbool.h>
#include <inttypes.h>

/* ============================================================================
 * Constants and Configuration
 * ============================================================================ */

#define HCS_VERSION "0.4.0"
#define MAX_ALLOCATIONS 65536
#define DEFAULT_QUOTA_GB 4

/* CUDA error codes */
#define cudaSuccess 0
#define cudaErrorMemoryAllocation 2
#define cudaErrorInvalidValue 1

/* ACL error codes (华为昇腾) */
#define ACL_SUCCESS 0
#define ACL_ERROR_RT_MEMORY_ALLOCATION 107000
#define ACL_ERROR_INVALID_PARAM 107001

/* HIP error codes (海光/AMD) */
#define hipSuccess 0
#define hipErrorOutOfMemory 2
#define hipErrorInvalidValue 1

/* ACL memory allocation policy */
typedef enum {
    ACL_MEM_MALLOC_HUGE_FIRST = 0,
    ACL_MEM_MALLOC_HUGE_ONLY = 1,
    ACL_MEM_MALLOC_NORMAL_ONLY = 2
} aclrtMemMallocPolicy;

/* ACL memory attribute for memory info query */
typedef enum {
    ACL_DDR_MEM = 0,
    ACL_HBM_MEM = 1,
    ACL_DDR_MEM_HUGE = 2,
    ACL_DDR_MEM_NORMAL = 3,
    ACL_HBM_MEM_HUGE = 4,
    ACL_HBM_MEM_NORMAL = 5
} aclrtMemAttr;

/* Log levels */
typedef enum {
    LOG_DEBUG = 0,
    LOG_INFO = 1,
    LOG_WARN = 2,
    LOG_ERROR = 3,
    LOG_NONE = 4
} log_level_t;

/* ============================================================================
 * Data Structures
 * ============================================================================ */

/* Memory allocation tracking entry */
typedef struct {
    void *ptr;
    size_t size;
    bool in_use;
} allocation_entry_t;

/* Global quota context */
typedef struct {
    pthread_mutex_t lock;

    /* Quota configuration */
    size_t quota_limit;
    size_t quota_used;

    /* Allocation tracking */
    allocation_entry_t allocations[MAX_ALLOCATIONS];
    int allocation_count;

    /* Statistics */
    size_t peak_usage;
    uint64_t total_allocs;
    uint64_t total_frees;
    uint64_t failed_allocs;

    /* Configuration */
    log_level_t log_level;
    bool initialized;
} quota_context_t;

/* ============================================================================
 * Global State
 * ============================================================================ */

static quota_context_t g_ctx = {
    .lock = PTHREAD_MUTEX_INITIALIZER,
    .quota_limit = 0,
    .quota_used = 0,
    .allocation_count = 0,
    .peak_usage = 0,
    .total_allocs = 0,
    .total_frees = 0,
    .failed_allocs = 0,
    .log_level = LOG_WARN,
    .initialized = false
};

/* Real CUDA function pointers */
typedef int (*cudaMalloc_fn)(void **devPtr, size_t size);
typedef int (*cudaFree_fn)(void *devPtr);
typedef int (*cudaMemGetInfo_fn)(size_t *free, size_t *total);
typedef int (*cudaMallocManaged_fn)(void **devPtr, size_t size, unsigned int flags);
typedef int (*cudaMallocHost_fn)(void **ptr, size_t size);
typedef int (*cudaFreeHost_fn)(void *ptr);

static cudaMalloc_fn real_cudaMalloc = NULL;
static cudaFree_fn real_cudaFree = NULL;
static cudaMemGetInfo_fn real_cudaMemGetInfo = NULL;
static cudaMallocManaged_fn real_cudaMallocManaged = NULL;
static cudaMallocHost_fn real_cudaMallocHost = NULL;
static cudaFreeHost_fn real_cudaFreeHost = NULL;

/* Real ACL function pointers (华为昇腾) */
typedef int (*aclrtMalloc_fn)(void **devPtr, size_t size, aclrtMemMallocPolicy policy);
typedef int (*aclrtFree_fn)(void *devPtr);
typedef int (*aclrtGetMemInfo_fn)(aclrtMemAttr attr, size_t *free, size_t *total);

static aclrtMalloc_fn real_aclrtMalloc = NULL;
static aclrtFree_fn real_aclrtFree = NULL;
static aclrtGetMemInfo_fn real_aclrtGetMemInfo = NULL;

/* Real HIP function pointers (海光/AMD) */
typedef int (*hipMalloc_fn)(void **devPtr, size_t size);
typedef int (*hipFree_fn)(void *devPtr);
typedef int (*hipMemGetInfo_fn)(size_t *free, size_t *total);

static hipMalloc_fn real_hipMalloc = NULL;
static hipFree_fn real_hipFree = NULL;
static hipMemGetInfo_fn real_hipMemGetInfo = NULL;

/* ============================================================================
 * Utility Functions
 * ============================================================================ */

static const char* log_level_str(log_level_t level) {
    switch (level) {
        case LOG_DEBUG: return "DEBUG";
        case LOG_INFO:  return "INFO";
        case LOG_WARN:  return "WARN";
        case LOG_ERROR: return "ERROR";
        default:        return "UNKNOWN";
    }
}

#define HCS_LOG(level, fmt, ...) do { \
    if (level >= g_ctx.log_level) { \
        fprintf(stderr, "[HCS %s] " fmt "\n", log_level_str(level), ##__VA_ARGS__); \
    } \
} while(0)

/* Parse size string like "16Gi", "4G", "1024Mi", "1024M", "1024" */
static size_t parse_size_string(const char *str) {
    if (!str || !*str) return 0;

    char *endptr;
    double value = strtod(str, &endptr);

    if (endptr == str) return 0;

    /* Skip whitespace */
    while (*endptr == ' ') endptr++;

    /* Parse unit suffix */
    if (strncasecmp(endptr, "Gi", 2) == 0 || strncasecmp(endptr, "GiB", 3) == 0) {
        value *= (1024.0 * 1024.0 * 1024.0);
    } else if (strncasecmp(endptr, "G", 1) == 0 || strncasecmp(endptr, "GB", 2) == 0) {
        value *= (1000.0 * 1000.0 * 1000.0);
    } else if (strncasecmp(endptr, "Mi", 2) == 0 || strncasecmp(endptr, "MiB", 3) == 0) {
        value *= (1024.0 * 1024.0);
    } else if (strncasecmp(endptr, "M", 1) == 0 || strncasecmp(endptr, "MB", 2) == 0) {
        value *= (1000.0 * 1000.0);
    } else if (strncasecmp(endptr, "Ki", 2) == 0 || strncasecmp(endptr, "KiB", 3) == 0) {
        value *= 1024.0;
    } else if (strncasecmp(endptr, "K", 1) == 0 || strncasecmp(endptr, "KB", 2) == 0) {
        value *= 1000.0;
    }
    /* else: assume bytes */

    return (size_t)value;
}

/* Format size for logging */
static void format_size(size_t bytes, char *buf, size_t buflen) {
    if (bytes >= (1024UL * 1024 * 1024)) {
        snprintf(buf, buflen, "%.2f GiB", (double)bytes / (1024.0 * 1024.0 * 1024.0));
    } else if (bytes >= (1024UL * 1024)) {
        snprintf(buf, buflen, "%.2f MiB", (double)bytes / (1024.0 * 1024.0));
    } else if (bytes >= 1024) {
        snprintf(buf, buflen, "%.2f KiB", (double)bytes / 1024.0);
    } else {
        snprintf(buf, buflen, "%zu B", bytes);
    }
}

/* Parse log level from string */
static log_level_t parse_log_level(const char *str) {
    if (!str) return LOG_WARN;

    if (strcasecmp(str, "debug") == 0) return LOG_DEBUG;
    if (strcasecmp(str, "info") == 0)  return LOG_INFO;
    if (strcasecmp(str, "warn") == 0)  return LOG_WARN;
    if (strcasecmp(str, "error") == 0) return LOG_ERROR;
    if (strcasecmp(str, "none") == 0)  return LOG_NONE;

    return LOG_WARN;
}

/* ============================================================================
 * Allocation Tracking
 * ============================================================================ */

/* Find allocation entry by pointer (must hold lock) */
static int find_allocation(void *ptr) {
    for (int i = 0; i < g_ctx.allocation_count; i++) {
        if (g_ctx.allocations[i].in_use && g_ctx.allocations[i].ptr == ptr) {
            return i;
        }
    }
    return -1;
}

/* Add allocation entry (must hold lock) */
static bool add_allocation(void *ptr, size_t size) {
    /* First, try to find an unused slot */
    for (int i = 0; i < g_ctx.allocation_count; i++) {
        if (!g_ctx.allocations[i].in_use) {
            g_ctx.allocations[i].ptr = ptr;
            g_ctx.allocations[i].size = size;
            g_ctx.allocations[i].in_use = true;
            return true;
        }
    }

    /* No unused slot, append if space available */
    if (g_ctx.allocation_count < MAX_ALLOCATIONS) {
        g_ctx.allocations[g_ctx.allocation_count].ptr = ptr;
        g_ctx.allocations[g_ctx.allocation_count].size = size;
        g_ctx.allocations[g_ctx.allocation_count].in_use = true;
        g_ctx.allocation_count++;
        return true;
    }

    return false;
}

/* Remove allocation entry (must hold lock) */
static size_t remove_allocation(void *ptr) {
    int idx = find_allocation(ptr);
    if (idx >= 0) {
        size_t size = g_ctx.allocations[idx].size;
        g_ctx.allocations[idx].in_use = false;
        return size;
    }
    return 0;
}

/* ============================================================================
 * Initialization
 * ============================================================================ */

static void load_real_functions(void) {
    /* CUDA Runtime API */
    real_cudaMalloc = (cudaMalloc_fn)dlsym(RTLD_NEXT, "cudaMalloc");
    real_cudaFree = (cudaFree_fn)dlsym(RTLD_NEXT, "cudaFree");
    real_cudaMemGetInfo = (cudaMemGetInfo_fn)dlsym(RTLD_NEXT, "cudaMemGetInfo");
    real_cudaMallocManaged = (cudaMallocManaged_fn)dlsym(RTLD_NEXT, "cudaMallocManaged");
    real_cudaMallocHost = (cudaMallocHost_fn)dlsym(RTLD_NEXT, "cudaMallocHost");
    real_cudaFreeHost = (cudaFreeHost_fn)dlsym(RTLD_NEXT, "cudaFreeHost");

    /* ACL Runtime API (华为昇腾) */
    real_aclrtMalloc = (aclrtMalloc_fn)dlsym(RTLD_NEXT, "aclrtMalloc");
    real_aclrtFree = (aclrtFree_fn)dlsym(RTLD_NEXT, "aclrtFree");
    real_aclrtGetMemInfo = (aclrtGetMemInfo_fn)dlsym(RTLD_NEXT, "aclrtGetMemInfo");

    /* HIP Runtime API (海光/AMD) */
    real_hipMalloc = (hipMalloc_fn)dlsym(RTLD_NEXT, "hipMalloc");
    real_hipFree = (hipFree_fn)dlsym(RTLD_NEXT, "hipFree");
    real_hipMemGetInfo = (hipMemGetInfo_fn)dlsym(RTLD_NEXT, "hipMemGetInfo");
}

__attribute__((constructor))
static void hcs_init(void) {
    if (g_ctx.initialized) return;

    /* Parse log level */
    g_ctx.log_level = parse_log_level(getenv("HCS_LOG_LEVEL"));

    /* Parse quota */
    const char *quota_str = getenv("HCS_VRAM_QUOTA");
    if (quota_str && *quota_str) {
        g_ctx.quota_limit = parse_size_string(quota_str);
    } else {
        /* Default: 4 GiB */
        g_ctx.quota_limit = (size_t)DEFAULT_QUOTA_GB * 1024 * 1024 * 1024;
    }

    /* Load real CUDA functions */
    load_real_functions();

    g_ctx.initialized = true;

    char quota_buf[32];
    format_size(g_ctx.quota_limit, quota_buf, sizeof(quota_buf));
    HCS_LOG(LOG_INFO, "HCS Interceptor v%s initialized, quota=%s", HCS_VERSION, quota_buf);

    if (!real_cudaMalloc) {
        HCS_LOG(LOG_WARN, "cudaMalloc not found - CUDA library may not be loaded yet");
    }
}

__attribute__((destructor))
static void hcs_cleanup(void) {
    if (!g_ctx.initialized) return;

    char used_buf[32], peak_buf[32], limit_buf[32];
    format_size(g_ctx.quota_used, used_buf, sizeof(used_buf));
    format_size(g_ctx.peak_usage, peak_buf, sizeof(peak_buf));
    format_size(g_ctx.quota_limit, limit_buf, sizeof(limit_buf));

    HCS_LOG(LOG_INFO, "HCS Interceptor shutdown: allocs=%" PRIu64 ", frees=%" PRIu64 ", failed=%" PRIu64 ", peak=%s, final=%s, limit=%s",
            g_ctx.total_allocs, g_ctx.total_frees, g_ctx.failed_allocs,
            peak_buf, used_buf, limit_buf);
}

/* ============================================================================
 * CUDA API Interception
 * ============================================================================ */

/* cudaMalloc interception */
int cudaMalloc(void **devPtr, size_t size) {
    /* Ensure initialization */
    if (!g_ctx.initialized) hcs_init();

    /* Lazy load real function */
    if (!real_cudaMalloc) {
        real_cudaMalloc = (cudaMalloc_fn)dlsym(RTLD_NEXT, "cudaMalloc");
        if (!real_cudaMalloc) {
            HCS_LOG(LOG_ERROR, "Failed to find real cudaMalloc");
            return cudaErrorInvalidValue;
        }
    }

    pthread_mutex_lock(&g_ctx.lock);

    /* Check quota */
    if (g_ctx.quota_used + size > g_ctx.quota_limit) {
        g_ctx.failed_allocs++;

        char req_buf[32], used_buf[32], limit_buf[32];
        format_size(size, req_buf, sizeof(req_buf));
        format_size(g_ctx.quota_used, used_buf, sizeof(used_buf));
        format_size(g_ctx.quota_limit, limit_buf, sizeof(limit_buf));

        HCS_LOG(LOG_WARN, "cudaMalloc DENIED: requested=%s, used=%s, limit=%s",
                req_buf, used_buf, limit_buf);

        pthread_mutex_unlock(&g_ctx.lock);
        return cudaErrorMemoryAllocation;
    }

    pthread_mutex_unlock(&g_ctx.lock);

    /* Call real cudaMalloc */
    int result = real_cudaMalloc(devPtr, size);

    if (result == cudaSuccess && devPtr && *devPtr) {
        pthread_mutex_lock(&g_ctx.lock);

        /* Update quota */
        g_ctx.quota_used += size;
        g_ctx.total_allocs++;

        /* Update peak */
        if (g_ctx.quota_used > g_ctx.peak_usage) {
            g_ctx.peak_usage = g_ctx.quota_used;
        }

        /* Track allocation */
        if (!add_allocation(*devPtr, size)) {
            HCS_LOG(LOG_WARN, "Failed to track allocation (table full)");
        }

        char size_buf[32], used_buf[32];
        format_size(size, size_buf, sizeof(size_buf));
        format_size(g_ctx.quota_used, used_buf, sizeof(used_buf));
        HCS_LOG(LOG_DEBUG, "cudaMalloc: size=%s, ptr=%p, total_used=%s",
                size_buf, *devPtr, used_buf);

        pthread_mutex_unlock(&g_ctx.lock);
    }

    return result;
}

/* cudaFree interception */
int cudaFree(void *devPtr) {
    /* Ensure initialization */
    if (!g_ctx.initialized) hcs_init();

    /* Lazy load real function */
    if (!real_cudaFree) {
        real_cudaFree = (cudaFree_fn)dlsym(RTLD_NEXT, "cudaFree");
        if (!real_cudaFree) {
            HCS_LOG(LOG_ERROR, "Failed to find real cudaFree");
            return cudaErrorInvalidValue;
        }
    }

    /* NULL pointer is valid for cudaFree */
    if (!devPtr) {
        return real_cudaFree(devPtr);
    }

    pthread_mutex_lock(&g_ctx.lock);

    /* Find and remove allocation */
    size_t size = remove_allocation(devPtr);
    if (size > 0) {
        if (g_ctx.quota_used >= size) {
            g_ctx.quota_used -= size;
        } else {
            g_ctx.quota_used = 0;
        }
        g_ctx.total_frees++;

        char size_buf[32], used_buf[32];
        format_size(size, size_buf, sizeof(size_buf));
        format_size(g_ctx.quota_used, used_buf, sizeof(used_buf));
        HCS_LOG(LOG_DEBUG, "cudaFree: size=%s, ptr=%p, total_used=%s",
                size_buf, devPtr, used_buf);
    } else {
        HCS_LOG(LOG_DEBUG, "cudaFree: ptr=%p (not tracked)", devPtr);
    }

    pthread_mutex_unlock(&g_ctx.lock);

    return real_cudaFree(devPtr);
}

/* cudaMemGetInfo interception - return virtualized memory info */
int cudaMemGetInfo(size_t *free, size_t *total) {
    /* Ensure initialization */
    if (!g_ctx.initialized) hcs_init();

    /* Lazy load real function */
    if (!real_cudaMemGetInfo) {
        real_cudaMemGetInfo = (cudaMemGetInfo_fn)dlsym(RTLD_NEXT, "cudaMemGetInfo");
        if (!real_cudaMemGetInfo) {
            HCS_LOG(LOG_ERROR, "Failed to find real cudaMemGetInfo");
            return cudaErrorInvalidValue;
        }
    }

    /* Get real memory info first to check for errors */
    int result = real_cudaMemGetInfo(free, total);
    if (result != cudaSuccess) {
        return result;
    }

    /* Return virtualized values */
    pthread_mutex_lock(&g_ctx.lock);

    *total = g_ctx.quota_limit;
    *free = (g_ctx.quota_limit > g_ctx.quota_used) ?
            (g_ctx.quota_limit - g_ctx.quota_used) : 0;

    char free_buf[32], total_buf[32];
    format_size(*free, free_buf, sizeof(free_buf));
    format_size(*total, total_buf, sizeof(total_buf));
    HCS_LOG(LOG_DEBUG, "cudaMemGetInfo: free=%s, total=%s (virtualized)",
            free_buf, total_buf);

    pthread_mutex_unlock(&g_ctx.lock);

    return cudaSuccess;
}

/* cudaMallocManaged interception */
int cudaMallocManaged(void **devPtr, size_t size, unsigned int flags) {
    /* Ensure initialization */
    if (!g_ctx.initialized) hcs_init();

    /* Lazy load real function */
    if (!real_cudaMallocManaged) {
        real_cudaMallocManaged = (cudaMallocManaged_fn)dlsym(RTLD_NEXT, "cudaMallocManaged");
        if (!real_cudaMallocManaged) {
            HCS_LOG(LOG_ERROR, "Failed to find real cudaMallocManaged");
            return cudaErrorInvalidValue;
        }
    }

    pthread_mutex_lock(&g_ctx.lock);

    /* Check quota */
    if (g_ctx.quota_used + size > g_ctx.quota_limit) {
        g_ctx.failed_allocs++;

        char req_buf[32], used_buf[32], limit_buf[32];
        format_size(size, req_buf, sizeof(req_buf));
        format_size(g_ctx.quota_used, used_buf, sizeof(used_buf));
        format_size(g_ctx.quota_limit, limit_buf, sizeof(limit_buf));

        HCS_LOG(LOG_WARN, "cudaMallocManaged DENIED: requested=%s, used=%s, limit=%s",
                req_buf, used_buf, limit_buf);

        pthread_mutex_unlock(&g_ctx.lock);
        return cudaErrorMemoryAllocation;
    }

    pthread_mutex_unlock(&g_ctx.lock);

    /* Call real cudaMallocManaged */
    int result = real_cudaMallocManaged(devPtr, size, flags);

    if (result == cudaSuccess && devPtr && *devPtr) {
        pthread_mutex_lock(&g_ctx.lock);

        g_ctx.quota_used += size;
        g_ctx.total_allocs++;

        if (g_ctx.quota_used > g_ctx.peak_usage) {
            g_ctx.peak_usage = g_ctx.quota_used;
        }

        if (!add_allocation(*devPtr, size)) {
            HCS_LOG(LOG_WARN, "Failed to track allocation (table full)");
        }

        char size_buf[32];
        format_size(size, size_buf, sizeof(size_buf));
        HCS_LOG(LOG_DEBUG, "cudaMallocManaged: size=%s, ptr=%p", size_buf, *devPtr);

        pthread_mutex_unlock(&g_ctx.lock);
    }

    return result;
}

/* ============================================================================
 * ACL API Interception (华为昇腾)
 * ============================================================================ */

/* aclrtMalloc interception */
int aclrtMalloc(void **devPtr, size_t size, aclrtMemMallocPolicy policy) {
    /* Ensure initialization */
    if (!g_ctx.initialized) hcs_init();

    /* Lazy load real function */
    if (!real_aclrtMalloc) {
        real_aclrtMalloc = (aclrtMalloc_fn)dlsym(RTLD_NEXT, "aclrtMalloc");
        if (!real_aclrtMalloc) {
            HCS_LOG(LOG_ERROR, "Failed to find real aclrtMalloc");
            return ACL_ERROR_INVALID_PARAM;
        }
    }

    pthread_mutex_lock(&g_ctx.lock);

    /* Check quota */
    if (g_ctx.quota_used + size > g_ctx.quota_limit) {
        g_ctx.failed_allocs++;

        char req_buf[32], used_buf[32], limit_buf[32];
        format_size(size, req_buf, sizeof(req_buf));
        format_size(g_ctx.quota_used, used_buf, sizeof(used_buf));
        format_size(g_ctx.quota_limit, limit_buf, sizeof(limit_buf));

        HCS_LOG(LOG_WARN, "aclrtMalloc DENIED: requested=%s, used=%s, limit=%s",
                req_buf, used_buf, limit_buf);

        pthread_mutex_unlock(&g_ctx.lock);
        return ACL_ERROR_RT_MEMORY_ALLOCATION;
    }

    pthread_mutex_unlock(&g_ctx.lock);

    /* Call real aclrtMalloc */
    int result = real_aclrtMalloc(devPtr, size, policy);

    if (result == ACL_SUCCESS && devPtr && *devPtr) {
        pthread_mutex_lock(&g_ctx.lock);

        /* Update quota */
        g_ctx.quota_used += size;
        g_ctx.total_allocs++;

        /* Update peak */
        if (g_ctx.quota_used > g_ctx.peak_usage) {
            g_ctx.peak_usage = g_ctx.quota_used;
        }

        /* Track allocation */
        if (!add_allocation(*devPtr, size)) {
            HCS_LOG(LOG_WARN, "Failed to track allocation (table full)");
        }

        char size_buf[32], used_buf[32];
        format_size(size, size_buf, sizeof(size_buf));
        format_size(g_ctx.quota_used, used_buf, sizeof(used_buf));
        HCS_LOG(LOG_DEBUG, "aclrtMalloc: size=%s, ptr=%p, total_used=%s",
                size_buf, *devPtr, used_buf);

        pthread_mutex_unlock(&g_ctx.lock);
    }

    return result;
}

/* aclrtFree interception */
int aclrtFree(void *devPtr) {
    /* Ensure initialization */
    if (!g_ctx.initialized) hcs_init();

    /* Lazy load real function */
    if (!real_aclrtFree) {
        real_aclrtFree = (aclrtFree_fn)dlsym(RTLD_NEXT, "aclrtFree");
        if (!real_aclrtFree) {
            HCS_LOG(LOG_ERROR, "Failed to find real aclrtFree");
            return ACL_ERROR_INVALID_PARAM;
        }
    }

    /* NULL pointer check */
    if (!devPtr) {
        return real_aclrtFree(devPtr);
    }

    pthread_mutex_lock(&g_ctx.lock);

    /* Find and remove allocation */
    size_t size = remove_allocation(devPtr);
    if (size > 0) {
        if (g_ctx.quota_used >= size) {
            g_ctx.quota_used -= size;
        } else {
            g_ctx.quota_used = 0;
        }
        g_ctx.total_frees++;

        char size_buf[32], used_buf[32];
        format_size(size, size_buf, sizeof(size_buf));
        format_size(g_ctx.quota_used, used_buf, sizeof(used_buf));
        HCS_LOG(LOG_DEBUG, "aclrtFree: size=%s, ptr=%p, total_used=%s",
                size_buf, devPtr, used_buf);
    } else {
        HCS_LOG(LOG_DEBUG, "aclrtFree: ptr=%p (not tracked)", devPtr);
    }

    pthread_mutex_unlock(&g_ctx.lock);

    return real_aclrtFree(devPtr);
}

/* aclrtGetMemInfo interception - return virtualized memory info */
int aclrtGetMemInfo(aclrtMemAttr attr, size_t *free, size_t *total) {
    /* Ensure initialization */
    if (!g_ctx.initialized) hcs_init();

    /* Lazy load real function */
    if (!real_aclrtGetMemInfo) {
        real_aclrtGetMemInfo = (aclrtGetMemInfo_fn)dlsym(RTLD_NEXT, "aclrtGetMemInfo");
        if (!real_aclrtGetMemInfo) {
            HCS_LOG(LOG_ERROR, "Failed to find real aclrtGetMemInfo");
            return ACL_ERROR_INVALID_PARAM;
        }
    }

    /* Get real memory info first to check for errors */
    int result = real_aclrtGetMemInfo(attr, free, total);
    if (result != ACL_SUCCESS) {
        return result;
    }

    /* Return virtualized values */
    pthread_mutex_lock(&g_ctx.lock);

    *total = g_ctx.quota_limit;
    *free = (g_ctx.quota_limit > g_ctx.quota_used) ?
            (g_ctx.quota_limit - g_ctx.quota_used) : 0;

    char free_buf[32], total_buf[32];
    format_size(*free, free_buf, sizeof(free_buf));
    format_size(*total, total_buf, sizeof(total_buf));
    HCS_LOG(LOG_DEBUG, "aclrtGetMemInfo: free=%s, total=%s (virtualized)",
            free_buf, total_buf);

    pthread_mutex_unlock(&g_ctx.lock);

    return ACL_SUCCESS;
}

/* ============================================================================
 * HIP API Interception (海光/AMD)
 * ============================================================================ */

/* hipMalloc interception */
int hipMalloc(void **devPtr, size_t size) {
    /* Ensure initialization */
    if (!g_ctx.initialized) hcs_init();

    /* Lazy load real function */
    if (!real_hipMalloc) {
        real_hipMalloc = (hipMalloc_fn)dlsym(RTLD_NEXT, "hipMalloc");
        if (!real_hipMalloc) {
            HCS_LOG(LOG_ERROR, "Failed to find real hipMalloc");
            return hipErrorInvalidValue;
        }
    }

    pthread_mutex_lock(&g_ctx.lock);

    /* Check quota */
    if (g_ctx.quota_used + size > g_ctx.quota_limit) {
        g_ctx.failed_allocs++;

        char req_buf[32], used_buf[32], limit_buf[32];
        format_size(size, req_buf, sizeof(req_buf));
        format_size(g_ctx.quota_used, used_buf, sizeof(used_buf));
        format_size(g_ctx.quota_limit, limit_buf, sizeof(limit_buf));

        HCS_LOG(LOG_WARN, "hipMalloc DENIED: requested=%s, used=%s, limit=%s",
                req_buf, used_buf, limit_buf);

        pthread_mutex_unlock(&g_ctx.lock);
        return hipErrorOutOfMemory;
    }

    pthread_mutex_unlock(&g_ctx.lock);

    /* Call real hipMalloc */
    int result = real_hipMalloc(devPtr, size);

    if (result == hipSuccess && devPtr && *devPtr) {
        pthread_mutex_lock(&g_ctx.lock);

        /* Update quota */
        g_ctx.quota_used += size;
        g_ctx.total_allocs++;

        /* Update peak */
        if (g_ctx.quota_used > g_ctx.peak_usage) {
            g_ctx.peak_usage = g_ctx.quota_used;
        }

        /* Track allocation */
        if (!add_allocation(*devPtr, size)) {
            HCS_LOG(LOG_WARN, "Failed to track allocation (table full)");
        }

        char size_buf[32], used_buf[32];
        format_size(size, size_buf, sizeof(size_buf));
        format_size(g_ctx.quota_used, used_buf, sizeof(used_buf));
        HCS_LOG(LOG_DEBUG, "hipMalloc: size=%s, ptr=%p, total_used=%s",
                size_buf, *devPtr, used_buf);

        pthread_mutex_unlock(&g_ctx.lock);
    }

    return result;
}

/* hipFree interception */
int hipFree(void *devPtr) {
    /* Ensure initialization */
    if (!g_ctx.initialized) hcs_init();

    /* Lazy load real function */
    if (!real_hipFree) {
        real_hipFree = (hipFree_fn)dlsym(RTLD_NEXT, "hipFree");
        if (!real_hipFree) {
            HCS_LOG(LOG_ERROR, "Failed to find real hipFree");
            return hipErrorInvalidValue;
        }
    }

    /* NULL pointer check */
    if (!devPtr) {
        return real_hipFree(devPtr);
    }

    pthread_mutex_lock(&g_ctx.lock);

    /* Find and remove allocation */
    size_t size = remove_allocation(devPtr);
    if (size > 0) {
        if (g_ctx.quota_used >= size) {
            g_ctx.quota_used -= size;
        } else {
            g_ctx.quota_used = 0;
        }
        g_ctx.total_frees++;

        char size_buf[32], used_buf[32];
        format_size(size, size_buf, sizeof(size_buf));
        format_size(g_ctx.quota_used, used_buf, sizeof(used_buf));
        HCS_LOG(LOG_DEBUG, "hipFree: size=%s, ptr=%p, total_used=%s",
                size_buf, devPtr, used_buf);
    } else {
        HCS_LOG(LOG_DEBUG, "hipFree: ptr=%p (not tracked)", devPtr);
    }

    pthread_mutex_unlock(&g_ctx.lock);

    return real_hipFree(devPtr);
}

/* hipMemGetInfo interception - return virtualized memory info */
int hipMemGetInfo(size_t *free, size_t *total) {
    /* Ensure initialization */
    if (!g_ctx.initialized) hcs_init();

    /* Lazy load real function */
    if (!real_hipMemGetInfo) {
        real_hipMemGetInfo = (hipMemGetInfo_fn)dlsym(RTLD_NEXT, "hipMemGetInfo");
        if (!real_hipMemGetInfo) {
            HCS_LOG(LOG_ERROR, "Failed to find real hipMemGetInfo");
            return hipErrorInvalidValue;
        }
    }

    /* Get real memory info first to check for errors */
    int result = real_hipMemGetInfo(free, total);
    if (result != hipSuccess) {
        return result;
    }

    /* Return virtualized values */
    pthread_mutex_lock(&g_ctx.lock);

    *total = g_ctx.quota_limit;
    *free = (g_ctx.quota_limit > g_ctx.quota_used) ?
            (g_ctx.quota_limit - g_ctx.quota_used) : 0;

    char free_buf[32], total_buf[32];
    format_size(*free, free_buf, sizeof(free_buf));
    format_size(*total, total_buf, sizeof(total_buf));
    HCS_LOG(LOG_DEBUG, "hipMemGetInfo: free=%s, total=%s (virtualized)",
            free_buf, total_buf);

    pthread_mutex_unlock(&g_ctx.lock);

    return hipSuccess;
}

/* ============================================================================
 * Query Functions (for external tools)
 * ============================================================================ */

/* Get current quota usage (exported for debugging) */
size_t hcs_get_quota_used(void) {
    pthread_mutex_lock(&g_ctx.lock);
    size_t used = g_ctx.quota_used;
    pthread_mutex_unlock(&g_ctx.lock);
    return used;
}

/* Get quota limit (exported for debugging) */
size_t hcs_get_quota_limit(void) {
    return g_ctx.quota_limit;
}

/* Get peak usage (exported for debugging) */
size_t hcs_get_peak_usage(void) {
    pthread_mutex_lock(&g_ctx.lock);
    size_t peak = g_ctx.peak_usage;
    pthread_mutex_unlock(&g_ctx.lock);
    return peak;
}

/* Get statistics (exported for debugging) */
void hcs_get_stats(uint64_t *allocs, uint64_t *frees, uint64_t *failed) {
    pthread_mutex_lock(&g_ctx.lock);
    if (allocs) *allocs = g_ctx.total_allocs;
    if (frees) *frees = g_ctx.total_frees;
    if (failed) *failed = g_ctx.failed_allocs;
    pthread_mutex_unlock(&g_ctx.lock);
}
