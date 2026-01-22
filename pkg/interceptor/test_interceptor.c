/*
 * HCS Interceptor Test Program
 *
 * Tests the VRAM quota enforcement functionality of libhcs_interceptor.so
 *
 * Build:
 *   With CUDA:    gcc -o test test_interceptor.c -I/usr/local/cuda/include -lcudart
 *   Mock mode:    gcc -DHCS_MOCK_CUDA -o test test_interceptor.c
 *
 * Run:
 *   HCS_VRAM_QUOTA=1Gi LD_PRELOAD=./libhcs_interceptor.so ./test
 *
 * Copyright (c) 2024 HCS Project
 * SPDX-License-Identifier: Apache-2.0
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

#ifdef HCS_MOCK_CUDA

/*
 * Mock CUDA API for testing without actual CUDA installation
 */

#define cudaSuccess 0
#define cudaErrorMemoryAllocation 2

typedef int cudaError_t;

/* Mock implementations - these will be intercepted by LD_PRELOAD */
static size_t mock_allocated = 0;
static size_t mock_total = 16UL * 1024 * 1024 * 1024;  /* 16 GiB */

cudaError_t cudaMalloc(void **devPtr, size_t size) {
    /* This will be intercepted by libhcs_interceptor.so */
    /* If not intercepted, just allocate regular memory for testing */
    *devPtr = malloc(size);
    if (*devPtr) {
        mock_allocated += size;
        return cudaSuccess;
    }
    return cudaErrorMemoryAllocation;
}

cudaError_t cudaFree(void *devPtr) {
    /* This will be intercepted by libhcs_interceptor.so */
    if (devPtr) {
        free(devPtr);
    }
    return cudaSuccess;
}

cudaError_t cudaMemGetInfo(size_t *free, size_t *total) {
    /* This will be intercepted by libhcs_interceptor.so */
    *total = mock_total;
    *free = mock_total - mock_allocated;
    return cudaSuccess;
}

const char* cudaGetErrorString(cudaError_t error) {
    switch (error) {
        case cudaSuccess: return "cudaSuccess";
        case cudaErrorMemoryAllocation: return "cudaErrorMemoryAllocation";
        default: return "Unknown error";
    }
}

#else

/* Use real CUDA */
#include <cuda_runtime.h>

#endif

/* ============================================================================
 * Test Utilities
 * ============================================================================ */

#define GiB (1024UL * 1024 * 1024)
#define MiB (1024UL * 1024)

static int tests_run = 0;
static int tests_passed = 0;

#define TEST_ASSERT(cond, msg) do { \
    tests_run++; \
    if (cond) { \
        printf("  [PASS] %s\n", msg); \
        tests_passed++; \
    } else { \
        printf("  [FAIL] %s\n", msg); \
    } \
} while(0)

static void format_size(size_t bytes, char *buf, size_t buflen) {
    if (bytes >= GiB) {
        snprintf(buf, buflen, "%.2f GiB", (double)bytes / GiB);
    } else if (bytes >= MiB) {
        snprintf(buf, buflen, "%.2f MiB", (double)bytes / MiB);
    } else {
        snprintf(buf, buflen, "%zu bytes", bytes);
    }
}

/* ============================================================================
 * Test Cases
 * ============================================================================ */

void test_basic_allocation(void) {
    printf("\n=== Test: Basic Allocation ===\n");

    void *ptr = NULL;
    cudaError_t err;

    /* Small allocation should succeed */
    err = cudaMalloc(&ptr, 100 * MiB);
    TEST_ASSERT(err == cudaSuccess, "100 MiB allocation succeeds");
    TEST_ASSERT(ptr != NULL, "Pointer is not NULL");

    if (ptr) {
        err = cudaFree(ptr);
        TEST_ASSERT(err == cudaSuccess, "Free succeeds");
        ptr = NULL;
    }
}

void test_quota_enforcement(void) {
    printf("\n=== Test: Quota Enforcement ===\n");

    void *ptr1 = NULL, *ptr2 = NULL;
    cudaError_t err;

    /* Check initial memory info */
    size_t free_mem, total_mem;
    err = cudaMemGetInfo(&free_mem, &total_mem);
    TEST_ASSERT(err == cudaSuccess, "cudaMemGetInfo succeeds");

    char free_buf[32], total_buf[32];
    format_size(free_mem, free_buf, sizeof(free_buf));
    format_size(total_mem, total_buf, sizeof(total_buf));
    printf("  Initial: free=%s, total=%s\n", free_buf, total_buf);

    /* Allocate 500 MiB (should succeed with 1 GiB quota) */
    err = cudaMalloc(&ptr1, 500 * MiB);
    TEST_ASSERT(err == cudaSuccess, "500 MiB allocation succeeds");

    /* Check memory after first allocation */
    err = cudaMemGetInfo(&free_mem, &total_mem);
    format_size(free_mem, free_buf, sizeof(free_buf));
    printf("  After 500 MiB: free=%s\n", free_buf);

    /* Try to allocate another 600 MiB (should fail - would exceed 1 GiB quota) */
    err = cudaMalloc(&ptr2, 600 * MiB);
    TEST_ASSERT(err == cudaErrorMemoryAllocation,
                "600 MiB allocation fails (quota exceeded)");
    TEST_ASSERT(ptr2 == NULL, "Pointer is NULL after failed allocation");

    /* Free first allocation */
    if (ptr1) {
        err = cudaFree(ptr1);
        TEST_ASSERT(err == cudaSuccess, "Free first allocation");
        ptr1 = NULL;
    }

    /* Now 600 MiB allocation should succeed */
    err = cudaMalloc(&ptr2, 600 * MiB);
    TEST_ASSERT(err == cudaSuccess, "600 MiB allocation succeeds after free");

    /* Cleanup */
    if (ptr2) {
        cudaFree(ptr2);
    }
}

void test_memory_info_virtualization(void) {
    printf("\n=== Test: Memory Info Virtualization ===\n");

    size_t free_mem, total_mem;
    cudaError_t err;

    /* Get memory info */
    err = cudaMemGetInfo(&free_mem, &total_mem);
    TEST_ASSERT(err == cudaSuccess, "cudaMemGetInfo succeeds");

    /* Total should be close to quota (1 GiB in our test) */
    char total_buf[32];
    format_size(total_mem, total_buf, sizeof(total_buf));
    printf("  Reported total: %s\n", total_buf);

    /* With HCS_VRAM_QUOTA=1Gi, total should be 1 GiB */
    TEST_ASSERT(total_mem == 1 * GiB || total_mem < 2 * GiB,
                "Total memory matches quota (approximately)");

    /* Free should be less than or equal to total */
    TEST_ASSERT(free_mem <= total_mem, "Free <= Total");
}

void test_multiple_allocations(void) {
    printf("\n=== Test: Multiple Small Allocations ===\n");

    #define NUM_ALLOCS 10
    void *ptrs[NUM_ALLOCS] = {0};
    cudaError_t err;
    int successful_allocs = 0;

    /* Allocate multiple 50 MiB blocks */
    for (int i = 0; i < NUM_ALLOCS; i++) {
        err = cudaMalloc(&ptrs[i], 50 * MiB);
        if (err == cudaSuccess && ptrs[i] != NULL) {
            successful_allocs++;
        } else {
            printf("  Allocation %d failed (expected with 1 GiB quota)\n", i + 1);
            break;
        }
    }

    printf("  Successful allocations: %d x 50 MiB = %d MiB\n",
           successful_allocs, successful_allocs * 50);

    /* With 1 GiB quota, we should be able to allocate at least 10 x 50 MiB */
    TEST_ASSERT(successful_allocs >= 10, "At least 10 allocations of 50 MiB");

    /* Free all */
    for (int i = 0; i < NUM_ALLOCS; i++) {
        if (ptrs[i]) {
            cudaFree(ptrs[i]);
        }
    }
}

void test_null_free(void) {
    printf("\n=== Test: NULL Free ===\n");

    cudaError_t err;

    /* Free NULL should succeed (standard CUDA behavior) */
    err = cudaFree(NULL);
    TEST_ASSERT(err == cudaSuccess, "cudaFree(NULL) succeeds");
}

/* ============================================================================
 * Main
 * ============================================================================ */

int main(int argc, char **argv) {
    (void)argc;
    (void)argv;

    printf("HCS Interceptor Test Suite\n");
    printf("==========================\n");

#ifdef HCS_MOCK_CUDA
    printf("Mode: MOCK CUDA (no real GPU required)\n");
#else
    printf("Mode: REAL CUDA\n");
#endif

    /* Check environment */
    const char *quota = getenv("HCS_VRAM_QUOTA");
    const char *preload = getenv("LD_PRELOAD");

    printf("HCS_VRAM_QUOTA: %s\n", quota ? quota : "(not set)");
    printf("LD_PRELOAD: %s\n", preload ? preload : "(not set)");

    if (!quota) {
        printf("\nWARNING: HCS_VRAM_QUOTA not set, using default quota\n");
    }

    if (!preload || !strstr(preload, "libhcs_interceptor")) {
        printf("\nWARNING: libhcs_interceptor.so may not be loaded via LD_PRELOAD\n");
        printf("Run with: LD_PRELOAD=./build/libhcs_interceptor.so ./test\n");
    }

    /* Run tests */
    test_basic_allocation();
    test_quota_enforcement();
    test_memory_info_virtualization();
    test_multiple_allocations();
    test_null_free();

    /* Summary */
    printf("\n==========================\n");
    printf("Tests: %d/%d passed\n", tests_passed, tests_run);

    return (tests_passed == tests_run) ? 0 : 1;
}
