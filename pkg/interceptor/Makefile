# HCS Interceptor Library Makefile
#
# Build the VRAM interception library for GPU memory quota enforcement.
#
# Supported APIs:
#   - NVIDIA CUDA: cudaMalloc, cudaFree, cudaMemGetInfo, cudaMallocManaged
#   - Huawei ACL:  aclrtMalloc, aclrtFree, aclrtGetMemInfo
#   - AMD/Hygon HIP: hipMalloc, hipFree, hipMemGetInfo
#
# Usage:
#   make              - Build the shared library
#   make test         - Build and run test program
#   make clean        - Remove build artifacts
#   make install      - Install to /usr/local/hcs/lib
#
# Environment:
#   CUDA_PATH         - Path to CUDA installation (default: /usr/local/cuda)
#   ACL_PATH          - Path to Ascend ACL installation (default: /usr/local/Ascend/ascend-toolkit/latest)
#   HIP_PATH          - Path to HIP installation (default: /opt/rocm)
#   DESTDIR           - Installation prefix (default: /usr/local/hcs)

CC := gcc
CFLAGS := -fPIC -O2 -Wall -Wextra -Werror -std=c11
LDFLAGS := -shared -ldl -lpthread

# Platform detection
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Darwin)
    # macOS
    LDFLAGS := -dynamiclib -ldl -lpthread
    LIB_EXT := dylib
else
    # Linux
    LDFLAGS := -shared -ldl -lpthread
    LIB_EXT := so
endif

# Output
BUILD_DIR := build
LIB_NAME := libhcs_interceptor.$(LIB_EXT)
LIB_PATH := $(BUILD_DIR)/$(LIB_NAME)

# CUDA (optional, for test program)
CUDA_PATH ?= /usr/local/cuda
CUDA_INCLUDE := $(CUDA_PATH)/include
CUDA_LIB := $(CUDA_PATH)/lib64

# Installation
DESTDIR ?= /usr/local/hcs
INSTALL_LIB_DIR := $(DESTDIR)/lib

# Source files
SRCS := libhcs_interceptor.c
OBJS := $(patsubst %.c,$(BUILD_DIR)/%.o,$(SRCS))

# Targets
.PHONY: all clean test install uninstall

all: $(LIB_PATH)

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(BUILD_DIR)/%.o: %.c | $(BUILD_DIR)
	$(CC) $(CFLAGS) -c $< -o $@

$(LIB_PATH): $(OBJS)
	$(CC) $(LDFLAGS) -o $@ $^
	@echo "Built $(LIB_PATH)"
	@echo "  Architecture: $(UNAME_M)"
	@echo "  Platform: $(UNAME_S)"

# Test program (requires CUDA)
TEST_SRC := test_interceptor.c
TEST_BIN := $(BUILD_DIR)/test_interceptor

test-build: $(LIB_PATH)
	@if [ -d "$(CUDA_PATH)" ]; then \
		echo "Building test program with CUDA support..."; \
		$(CC) $(CFLAGS) -I$(CUDA_INCLUDE) -o $(TEST_BIN) $(TEST_SRC) \
			-L$(CUDA_LIB) -lcudart -Wl,-rpath,$(CUDA_LIB); \
	else \
		echo "CUDA not found at $(CUDA_PATH), building mock test..."; \
		$(CC) $(CFLAGS) -DHCS_MOCK_CUDA -o $(TEST_BIN) $(TEST_SRC); \
	fi

test: $(LIB_PATH) test-build
	@echo "Running interceptor test..."
	@echo "HCS_VRAM_QUOTA=1Gi LD_PRELOAD=$(LIB_PATH) $(TEST_BIN)"
	@HCS_VRAM_QUOTA=1Gi HCS_LOG_LEVEL=debug LD_PRELOAD=$(LIB_PATH) $(TEST_BIN) || true

# Mock test (no CUDA required)
test-mock: $(LIB_PATH)
	@echo "Running mock test..."
	$(CC) $(CFLAGS) -DHCS_MOCK_CUDA -o $(TEST_BIN) $(TEST_SRC)
	HCS_VRAM_QUOTA=1Gi HCS_LOG_LEVEL=debug LD_PRELOAD=$(LIB_PATH) $(TEST_BIN)

# Unit test for size parsing
test-parse: $(BUILD_DIR)
	$(CC) $(CFLAGS) -DTEST_PARSE_SIZE -o $(BUILD_DIR)/test_parse libhcs_interceptor.c -ldl -lpthread
	$(BUILD_DIR)/test_parse

clean:
	rm -rf $(BUILD_DIR)
	rm -f *.o *.so *.dylib

install: $(LIB_PATH)
	install -d $(INSTALL_LIB_DIR)
	install -m 755 $(LIB_PATH) $(INSTALL_LIB_DIR)/
	@echo "Installed to $(INSTALL_LIB_DIR)/$(LIB_NAME)"

uninstall:
	rm -f $(INSTALL_LIB_DIR)/$(LIB_NAME)

# Print configuration
info:
	@echo "HCS Interceptor Build Configuration"
	@echo "===================================="
	@echo "Platform:     $(UNAME_S)"
	@echo "Architecture: $(UNAME_M)"
	@echo "CC:           $(CC)"
	@echo "CFLAGS:       $(CFLAGS)"
	@echo "LDFLAGS:      $(LDFLAGS)"
	@echo "CUDA_PATH:    $(CUDA_PATH)"
	@echo "DESTDIR:      $(DESTDIR)"
	@echo "Output:       $(LIB_PATH)"
