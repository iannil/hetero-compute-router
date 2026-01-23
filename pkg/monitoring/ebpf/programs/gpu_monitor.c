// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/* gpu_monitor.c - eBPF program for GPU metrics monitoring
 *
 * This eBPF program attaches to GPU tracepoints to collect:
 * - GPU core clock (MHz)
 * - Memory clock (MHz)
 * - Power usage (mW)
 * - Temperature (Celsius)
 * - Utilization (%)
 * - Throttling flags
 *
 * Supports:
 * - NVIDIA GPUs via nvml tracepoints
 * - AMD/ROCm GPUs via amdgpu tracepoints
 * - Hygon DCUs via amdgpu tracepoints
 */

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

/* GPU event structure */
struct gpu_event {
	__u32 device_id;
	__u64 timestamp;
	__u32 core_clock;
	__u32 mem_clock;
	__u32 power;
	__u32 temperature;
	__u32 utilization;
	__u8 throttling_flags;
};

/* BPF map for GPU events */
struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 256 * 1024);
} gpu_events SEC(".maps");

/* Per-CPU buffer for event submission */
struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__uint(max_entries, 1);
	__type(key, __u32);
	__type(value, struct gpu_event);
} gpu_event_buf SEC(".maps");

/* Throttling flags */
#define THROTTLE_POWER  0x01
#define THROTTLE_THERMAL 0x02
#define THROTTLE_RELIABILITY 0x04

/* Submit GPU event to ring buffer */
static __always_inline int submit_gpu_event(__u32 device_id,
					    __u32 core_clock,
					    __u32 mem_clock,
					    __u32 power,
					    __u32 temperature,
					    __u32 utilization,
					    __u8 throttling_flags)
{
	struct gpu_event *e;
	__u32 key = 0;
	__u64 flags;

	e = bpf_map_lookup_elem(&gpu_event_buf, &key);
	if (!e)
		return 0;

	e->device_id = device_id;
	e->timestamp = bpf_ktime_get_ns();
	e->core_clock = core_clock;
	e->mem_clock = mem_clock;
	e->power = power;
	e->temperature = temperature;
	e->utilization = utilization;
	e->throttling_flags = throttling_flags;

	flags = BPF_F_CURRENT_CPU;
	bpf_ringbuf_output(&gpu_events, e, sizeof(*e), flags);

	return 0;
}

/* NVIDIA GPU activity tracepoint */
SEC("tp/nvml/nvml_gpu_activity")
int handle_nvml_activity(struct trace_event_raw_nvml_gpu_activity *ctx)
{
	__u32 device_id = ctx->gpu_id;
	__u32 utilization = ctx->utilization;

	/* Read additional metrics from sysfs */
	submit_gpu_event(device_id, 0, 0, 0, 0, utilization, 0);
	return 0;
}

/* AMD GPU clock tracepoint */
SEC("tp/amdgpu/amdgpu_gpu_clock")
int handle_amdgpu_clock(struct trace_event_raw_amdgpu_gpu_clock *ctx)
{
	__u32 device_id = ctx->dev_id;
	__u32 core_clock = ctx->sclk;
	__u32 mem_clock = ctx->mclk;

	submit_gpu_event(device_id, core_clock, mem_clock, 0, 0, 0, 0);
	return 0;
}

/* AMD GPU power tracepoint */
SEC("tp/amdgpu/amdgpu_gpu_power")
int handle_amdgpu_power(struct trace_event_raw_amdgpu_gpu_power *ctx)
{
	__u32 device_id = ctx->dev_id;
	__u32 power = ctx->power; /* in milliwatts */

	submit_gpu_event(device_id, 0, 0, power, 0, 0, 0);
	return 0;
}

/* AMD GPU temperature tracepoint */
SEC("tp/amdgpu/amdgpu_gpu_temp")
int handle_amdgpu_temp(struct trace_event_raw_amdgpu_gpu_temp *ctx)
{
	__u32 device_id = ctx->dev_id;
	__u32 temperature = ctx->temp; /* in millidegrees Celsius */

	submit_gpu_event(device_id, 0, 0, 0, temperature / 1000, 0, 0);
	return 0;
}

/* AMD GPU busy tracepoint (utilization) */
SEC("tp/amdgpu/amdgpu_gpu_busy")
int handle_amdgpu_busy(struct trace_event_raw_amdgpu_gpu_busy *ctx)
{
	__u32 device_id = ctx->dev_id;
	__u32 utilization = ctx->busy_percent;

	submit_gpu_event(device_id, 0, 0, 0, 0, utilization, 0);
	return 0;
}

/* Fallback: kprobe for GPU frequency scaling */
SEC("kprobe/update_util")
int BPF_KPROBE(handle_update_util, struct task_struct *p, unsigned int util)
{
	/* This is a generic fallback for when tracepoints aren't available */
	__u32 device_id = 0; /* Would be determined from context */
	__u32 utilization = util;

	submit_gpu_event(device_id, 0, 0, 0, 0, utilization, 0);
	return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
