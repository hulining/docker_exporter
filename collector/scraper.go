package collector

import (
	"context"

	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	Scrapers = map[Scraper]bool{
		&InfoScraper{}:      true,
		&ContainerScraper{}: true,
		&ImageScraper{}:     true,
	}
)

type Scraper interface {
	// Name of the Scraper. Should be unique.
	Name() string

	// Help describes the role of the Scraper.
	// Example: "Collect from SHOW ENGINE INNODB STATUS"
	Help() string

	// Scrape collects data from client and sends it over channel as prometheus metric.
	Scrape(ctx context.Context, client *client.Client, ch chan<- prometheus.Metric) error
}
