// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/* health_events.c - eBPF program for GPU health event monitoring
 *
 * This eBPF program monitors GPU health events:
 * - ECC single-bit errors
 * - ECC double-bit errors
 * - Page retirement events
 * - GPU reset events
 * - Thermal throttling events
 * - Power throttling events
 */

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

/* Health event types */
#define EVENT_ECC_SB      0  /* Single-bit ECC error */
#define EVENT_ECC_DB      1  /* Double-bit ECC error */
#define EVENT_PAGE_RETIRE 2  /* Memory page retirement */
#define EVENT_GPU_RESET   3  /* GPU reset */
#define EVENT_THROTTLE_THERM 4 /* Thermal throttling */
#define EVENT_THROTTLE_POWER  5 /* Power throttling */

/* Health event structure */
struct health_event {
	__u32 device_id;
	__u64 timestamp;
	__u8 event_type;
	__u32 count;
	__u64 address;
};

/* BPF map for health events */
struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 256 * 1024);
} health_events SEC(".maps");

/* Per-CPU buffer for event submission */
struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__uint(max_entries, 1);
	__type(key, __u32);
	__type(value, struct health_event);
} health_event_buf SEC(".maps");

/* Submit health event to ring buffer */
static __always_inline int submit_health_event(__u32 device_id,
					      __u8 event_type,
					      __u32 count,
					      __u64 address)
{
	struct health_event *e;
	__u32 key = 0;
	__u64 flags;

	e = bpf_map_lookup_elem(&health_event_buf, &key);
	if (!e)
		return 0;

	e->device_id = device_id;
	e->timestamp = bpf_ktime_get_ns();
	e->event_type = event_type;
	e->count = count;
	e->address = address;

	flags = BPF_F_CURRENT_CPU;
	bpf_ringbuf_output(&health_events, e, sizeof(*e), flags);

	return 0;
}

/* NVIDIA ECC error tracepoint */
SEC("tp/nvml/nvml_ecc_error")
int handle_nvml_ecc(struct trace_event_raw_nvml_ecc_error *ctx)
{
	__u32 device_id = ctx->gpu_id;
	__u8 event_type;
	__u64 address = ctx->address;

	if (ctx->error_type == 0) {
		event_type = EVENT_ECC_SB;
	} else {
		event_type = EVENT_ECC_DB;
	}

	submit_health_event(device_id, event_type, 1, address);
	return 0;
}

/* AMD GPU ECC error tracepoint */
SEC("tp/amdgpu/amdgpu_ecc_error")
int handle_amdgpu_ecc(struct trace_event_raw_amdgpu_ecc_error *ctx)
{
	__u32 device_id = ctx->dev_id;
	__u8 event_type;
	__u64 address = ctx->address;

	if (ctx->error_type == 0) {
		event_type = EVENT_ECC_SB;
	} else {
		event_type = EVENT_ECC_DB;
	}

	submit_health_event(device_id, event_type, 1, address);
	return 0;
}

/* GPU reset event tracepoint */
SEC("tp/nvml/nvml_gpu_reset")
int handle_nvml_reset(struct trace_event_raw_nvml_gpu_reset *ctx)
{
	__u32 device_id = ctx->gpu_id;

	submit_health_event(device_id, EVENT_GPU_RESET, 1, 0);
	return 0;
}

/* AMD GPU reset tracepoint */
SEC("tp/amdgpu/amdgpu_gpu_reset")
int handle_amdgpu_reset(struct trace_event_raw_amdgpu_gpu_reset *ctx)
{
	__u32 device_id = ctx->dev_id;

	submit_health_event(device_id, EVENT_GPU_RESET, 1, 0);
	return 0;
}

/* Memory page retirement (bad memory pages) */
SEC("tp/amdgpu/amdgpu_bad_page")
int handle_amdgpu_bad_page(struct trace_event_raw_amdgpu_bad_page *ctx)
{
	__u32 device_id = ctx->dev_id;
	__u64 address = ctx->page_address;

	submit_health_event(device_id, EVENT_PAGE_RETIRE, 1, address);
	return 0;
}

/* Thermal throttling detection via temperature monitoring */
SEC("tp/thermal/thermal_temperature_trip")
int handle_thermal_trip(struct trace_event_raw_thermal_temperature_trip *ctx)
{
	/* Check if this is a GPU thermal zone */
	char tz_name[16];

	bpf_probe_read_kernel_str(tz_name, sizeof(tz_name), ctx->tz_name);

	/* Check for GPU-related thermal zones */
	if (bpf_strstr(tz_name, "gpu") ||
	    bpf_strstr(tz_name, "GPU") ||
	    bpf_strstr(tz_name, "amdgpu") ||
	    bpf_strstr(tz_name, "nvidia")) {
		__u32 device_id = 0; /* Would parse from thermal zone name */
		submit_health_event(device_id, EVENT_THROTTLE_THERM, 1, 0);
	}

	return 0;
}

/* RAPL (Running Average Power Limit) events for power throttling */
SEC("tp/power/energy_threshold")
int handle_power_threshold(struct trace_event_raw_power_energy_threshold *ctx)
{
	char domain_name[16];

	bpf_probe_read_kernel_str(domain_name, sizeof(domain_name),
				  ctx->domain_name);

	/* Check for GPU power domains */
	if (bpf_strstr(domain_name, "gpu") ||
	    bpf_strstr(domain_name, "GPU")) {
		__u32 device_id = 0; /* Would parse from domain name */
		submit_health_event(device_id, EVENT_THROTTLE_POWER, 1, 0);
	}

	return 0;
}

/* Fallback: kprobe for memory failure handling */
SEC("kprobe/memory_failure")
int BPF_KPROBE(handle_memory_failure, unsigned long pfn, int flags)
{
	/* This is called when a memory error is detected */
	/* Could be mapped to GPU memory if the PFN range matches GPU BAR */

	__u32 device_id = 0; /* Would need PFN to device mapping */
	__u64 address = pfn << PAGE_SHIFT;

	submit_health_event(device_id, EVENT_ECC_DB, 1, address);
	return 0;
}

/* kprobe: gpu recover - GPU recovery attempt */
SEC("kprobe/amdgpu_device_gpu_recover")
int BPF_KPROBE(handle_amdgpu_recover, struct amdgpu_device *adev)
{
	__u32 device_id = 0;

	if (adev) {
		/* Try to get device ID from adev structure */
		struct pci_dev *pdev = BPF_CORE_READ(adev, pdev);
		if (pdev) {
			device_id = BPF_CORE_READ(pdev, devfn);
		}
	}

	submit_health_event(device_id, EVENT_GPU_RESET, 1, 0);
	return 0;
}

/* MCE (Machine Check Exception) monitoring for x86 */
SEC("tp/mce/mce_record")
int handle_mce_record(struct trace_event_raw_mce_record *ctx)
{
	/* Check if MCE is from GPU memory (requires address range matching) */
	__u64 address = ctx->addr;
	__u8 status = ctx->status;

	/* Check if this is a memory error */
	if (status & 0x800) {
		__u32 device_id = 0; /* Would need address to device mapping */

		if (status & 0x40) {
			/* Uncorrected error */
			submit_health_event(device_id, EVENT_ECC_DB, 1, address);
		} else {
			/* Corrected error */
			submit_health_event(device_id, EVENT_ECC_SB, 1, address);
		}
	}

	return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
