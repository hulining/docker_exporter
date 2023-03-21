package collector

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var container_labels = []string{"name"}

var (
	restart_count = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, container_subnamespace, "restart_count"),
		"Number of times the runtime has restarted this container without explicit user action, since the container was last started.",
		container_labels, nil,
	)
	running_state = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, container_subnamespace, "running_state"),
		"Whether the container is running (1), restarting (0.5) or stopped (0).",
		container_labels, nil,
	)
	start_time_seconds = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, container_subnamespace, "start_time_seconds"),
		"Timestamp indicating when the container was started. Does not get reset by automatic restarts.",
		container_labels, nil,
	)
	inspect_duration_seconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "probe_inspect_duration_seconds",
		Help:      "How long it takes to query Docker for the basic information about a single container. Includes failed requests.",
		Buckets:   DurationBuckets,
	})
)

var (
	cpu_used_total = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, container_subnamespace, "cpu_used_total"),
		"Accumulated CPU usage of a container, in unspecified units, averaged for all logical CPUs usable by the container.",
		container_labels, nil,
	)
	cpu_capacity_total = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, container_subnamespace, "cpu_capacity_total"),
		"All potential CPU usage available to a container, in unspecified units, averaged for all logical CPUs usable by the container. Start point of measurement is undefined - only relative values should be used in analytics.",
		container_labels, nil,
	)
	memory_used_bytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, container_subnamespace, "memory_used_bytes"),
		"Memory usage of a container.",
		container_labels, nil,
	)
	network_in_bytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, container_subnamespace, "network_in_bytes"),
		"Total bytes received by the container's network interfaces.",
		container_labels, nil,
	)
	network_out_bytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, container_subnamespace, "network_out_bytes"),
		"Total bytes sent by the container's network interfaces.",
		container_labels, nil,
	)
	disk_read_bytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, container_subnamespace, "disk_read_bytes"),
		"Total bytes read from disk by a container.",
		container_labels, nil,
	)
	disk_write_bytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, container_subnamespace, "disk_write_bytes"),
		"Total bytes written to disk by a container.",
		container_labels, nil,
	)
)

type ContainerScraper struct{}

// Name of the Scraper. Should be unique.
func (s *ContainerScraper) Name() string {
	return "container"
}

// Help describes the role of the Scraper.
func (s *ContainerScraper) Help() string {
	return "Collect the container status & resource"
}

func (s *ContainerScraper) Scrape(ctx context.Context, client *client.Client, ch chan<- prometheus.Metric) error {

	containers, err := client.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	now := time.Now()
	for _, c := range containers {
		wg.Add(1)
		//go collectResource(&wg, ctx, client, c.ID, ch)
		go collectStatus(&wg, ctx, client, c.ID, ch)
	}
	wg.Wait()
	inspect_duration_seconds.Observe(time.Now().Sub(now).Seconds())
	ch <- inspect_duration_seconds

	return nil
}

var _ Scraper = &ContainerScraper{}

func collectStatus(wg *sync.WaitGroup, ctx context.Context, client *client.Client, id string, ch chan<- prometheus.Metric) {
	defer wg.Done()
	container, err := client.ContainerInspect(ctx, id)
	if err != nil {
		log.Errorf("inspect container %s err: %s", id[:12], err)
	}
	name := getNameByInspect(container)
	ch <- prometheus.MustNewConstMetric(restart_count, prometheus.GaugeValue, float64(container.RestartCount), name)

	if container.State.Running {
		ch <- prometheus.MustNewConstMetric(running_state, prometheus.GaugeValue, float64(1), name)
		if timestamp, err := time.Parse(time.RFC3339Nano, container.State.StartedAt); err == nil {
			ch <- prometheus.MustNewConstMetric(start_time_seconds, prometheus.GaugeValue, float64(timestamp.Unix()), name)
		}
		collectResource(ctx, client, id, ch)
	} else if container.State.Restarting {
		ch <- prometheus.MustNewConstMetric(running_state, prometheus.GaugeValue, float64(0.5), name)
	} else {
		ch <- prometheus.MustNewConstMetric(running_state, prometheus.GaugeValue, float64(0), name)
	}

}

func collectResource(ctx context.Context, client *client.Client, id string, ch chan<- prometheus.Metric) {
	res, err := client.ContainerStats(ctx, id, false)
	if err != nil {
		log.Errorf("inspect container %s err: %s", id[:12], err)
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorf("read container status %s err: %s", id[:12], err)
		return
	}
	var stats types.StatsJSON
	err = json.Unmarshal(body, &stats)
	if err != nil {
		log.Errorf("response body container status %s err: %s", id[:12], err)
		return
	}
	name := getNameByStatus(stats)

	ch <- prometheus.MustNewConstMetric(cpu_used_total, prometheus.GaugeValue, float64(stats.CPUStats.CPUUsage.TotalUsage), name)
	ch <- prometheus.MustNewConstMetric(cpu_capacity_total, prometheus.GaugeValue, float64(stats.CPUStats.SystemUsage), name)
	ch <- prometheus.MustNewConstMetric(memory_used_bytes, prometheus.GaugeValue, float64(stats.MemoryStats.Usage), name)

	if stats.Networks == nil {
		ch <- prometheus.MustNewConstMetric(network_in_bytes, prometheus.GaugeValue, float64(0), name)
		ch <- prometheus.MustNewConstMetric(network_out_bytes, prometheus.GaugeValue, float64(0), name)
	} else {
		in := uint64(0)
		out := uint64(0)
		for _, networkStats := range stats.Networks {
			in += networkStats.RxBytes
			out += networkStats.TxBytes
		}
		ch <- prometheus.MustNewConstMetric(network_in_bytes, prometheus.GaugeValue, float64(in), name)
		ch <- prometheus.MustNewConstMetric(network_out_bytes, prometheus.GaugeValue, float64(out), name)
	}
	ioEntrys := stats.BlkioStats.IoServiceBytesRecursive
	if len(ioEntrys) != 0 {
		read := uint64(0)
		write := uint64(0)

		for _, entry := range ioEntrys {
			if entry.Op == "read" {
				read += entry.Value
			}
			if entry.Op == "write" {
				write += entry.Value
			}
		}
		ch <- prometheus.MustNewConstMetric(disk_read_bytes, prometheus.GaugeValue, float64(read), name)
		ch <- prometheus.MustNewConstMetric(disk_write_bytes, prometheus.GaugeValue, float64(write), name)
	}
}
