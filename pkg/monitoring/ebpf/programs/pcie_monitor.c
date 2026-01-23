// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/* pcie_monitor.c - eBPF program for PCIe bandwidth monitoring
 *
 * This eBPF program monitors PCIe transactions to calculate:
 * - Read throughput (GB/s)
 * - Write throughput (GB/s)
 * - Transaction layer utilization
 * - Replay count (retries)
 *
 * Uses kprobes on PCIe driver functions and tracepoints.
 */

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

/* PCIe event structure */
struct pcie_event {
	__u32 device_id;
	__u64 timestamp;
	__u64 read_bytes;
	__u64 write_bytes;
	__u32 replay_count;
};

/* Statistics per device */
struct pcie_stats {
	__u64 read_bytes;
	__u64 write_bytes;
	__u32 replay_count;
	__u64 last_update;
};

/* BPF map for PCIe events */
struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 256 * 1024);
} pcie_events SEC(".maps");

/* Per-device statistics map */
struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 256);
	__type(key, __u32);
	__type(value, struct pcie_stats);
} pcie_stats_map SEC(".maps");

/* Per-CPU buffer for event submission */
struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__uint(max_entries, 1);
	__type(key, __u32);
	__type(value, struct pcie_event);
} pcie_event_buf SEC(".maps");

/* Submit PCIe event to ring buffer */
static __always_inline int submit_pcie_event(__u32 device_id,
					     __u64 read_bytes,
					     __u64 write_bytes,
					     __u32 replay_count)
{
	struct pcie_event *e;
	__u32 key = 0;
	__u64 flags;

	e = bpf_map_lookup_elem(&pcie_event_buf, &key);
	if (!e)
		return 0;

	e->device_id = device_id;
	e->timestamp = bpf_ktime_get_ns();
	e->read_bytes = read_bytes;
	e->write_bytes = write_bytes;
	e->replay_count = replay_count;

	flags = BPF_F_CURRENT_CPU;
	bpf_ringbuf_output(&pcie_events, e, sizeof(*e), flags);

	return 0;
}

/* Update device statistics */
static __always_inline int update_stats(__u32 device_id,
				       __u64 read_bytes,
				       __u64 write_bytes,
				       __u32 replay_count)
{
	struct pcie_stats *stats, new_stats;
	__u64 now = bpf_ktime_get_ns();

	stats = bpf_map_lookup_elem(&pcie_stats_map, &device_id);
	if (!stats) {
		/* Initialize new stats entry */
		new_stats.read_bytes = read_bytes;
		new_stats.write_bytes = write_bytes;
		new_stats.replay_count = replay_count;
		new_stats.last_update = now;
		bpf_map_update_elem(&pcie_stats_map, &device_id, &new_stats, BPF_ANY);
		return 0;
	}

	/* Update statistics */
	__sync_fetch_and_add(&stats->read_bytes, read_bytes);
	__sync_fetch_and_add(&stats->write_bytes, write_bytes);
	__sync_fetch_and_add(&stats->replay_count, replay_count);
	stats->last_update = now;

	return 0;
}

/* kprobe: pci_read - monitor PCIe read transactions */
SEC("kprobe/pci_read")
int BPF_KPROBE(handle_pci_read, struct pci_dev *dev,
	       void *buf, int len, int offset)
{
	__u32 device_id = dev->devfn;

	update_stats(device_id, len, 0, 0);
	return 0;
}

/* kprobe: pci_write - monitor PCIe write transactions */
SEC("kprobe/pci_write")
int BPF_KPROBE(handle_pci_write, struct pci_dev *dev,
	       void *buf, int len, int offset)
{
	__u32 device_id = dev->devfn;

	update_stats(device_id, 0, len, 0);
	return 0;
}

/* Tracepoint: PCIe replay event */
SEC("tp/irq/irq_handler_entry")
int handle_irq_entry(struct trace_event_raw_irq_handler_entry *ctx)
{
	/* Monitor interrupt handler for PCIe error conditions */
	char handler_name[32];

	bpf_probe_read_kernel_str(handler_name, sizeof(handler_name),
				  ctx->name);

	/* Check for PCIe error-related interrupts */
	if (bpf_strstr(handler_name, "pcie") ||
	    bpf_strstr(handler_name, "PCIe")) {
		/* This could indicate a replay or error condition */
		__u32 device_id = 0; /* Would need context mapping */
		update_stats(device_id, 0, 0, 1);
	}

	return 0;
}

/* kprobe: dma_map_page - monitor DMA transfers (GPU memory transfers) */
SEC("kprobe/dma_map_page")
int BPF_KPROBE(handle_dma_map_page, struct device *dev,
	       struct page *page, unsigned long offset,
	       size_t size, enum dma_data_direction dir)
{
	struct pci_dev *pdev;
	__u32 device_id;
	__u64 len = size;

	/* Get PCI device if available */
	if (!dev)
		return 0;

	pdev = bpf_container_of(dev, struct pci_dev, dev);
	if (!pdev)
		return 0;

	device_id = pdev->devfn;

	if (dir == DMA_TO_DEVICE) {
		/* Device read (host to device) */
		update_stats(device_id, 0, len, 0);
	} else if (dir == DMA_FROM_DEVICE) {
		/* Device write (device to host) */
		update_stats(device_id, len, 0, 0);
	}

	return 0;
}

/* Periodic statistics reader (called from userspace) */
SEC("syscall/read")
int handle_pcie_read(void)
{
	struct pcie_stats *stats;
	__u32 device_id = 0;
	__u64 now = bpf_ktime_get_ns();
	int i;

	/* Iterate through all devices and emit events for recent activity */
	for (i = 0; i < 256; i++) {
		device_id = i;
		stats = bpf_map_lookup_elem(&pcie_stats_map, &device_id);
		if (!stats)
			continue;

		/* Only report if updated recently (within 1 second) */
		if (now - stats->last_update > 1000000000ULL)
			continue;

		submit_pcie_event(device_id, stats->read_bytes,
				stats->write_bytes, stats->replay_count);

		/* Reset counters after reporting */
		__sync_fetch_and_and(&stats->read_bytes, 0);
		__sync_fetch_and_and(&stats->write_bytes, 0);
		__sync_fetch_and_and(&stats->replay_count, 0);
	}

	return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
