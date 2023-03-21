package collector

import (
	"context"

	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	containers = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "containers"),
		"Number of containers that exist.",
		[]string{"status"}, nil,
	)
)

type InfoScraper struct{}

// Name of the Scraper. Should be unique.
func (s *InfoScraper) Name() string {
	return "info"
}

// Help describes the role of the Scraper.
func (s *InfoScraper) Help() string {
	return "Collect the server info"
}

func (s *InfoScraper) Scrape(ctx context.Context, client *client.Client, ch chan<- prometheus.Metric) error {
	info, err := client.Info(ctx)
	if err != nil {
		log.Error(err)
	}

	ch <- prometheus.MustNewConstMetric(containers, prometheus.GaugeValue, float64(info.Containers), "total")
	ch <- prometheus.MustNewConstMetric(containers, prometheus.GaugeValue, float64(info.ContainersRunning), "running")
	ch <- prometheus.MustNewConstMetric(containers, prometheus.GaugeValue, float64(info.ContainersStopped), "stopped")

	return nil
}

var _ Scraper = &InfoScraper{}
