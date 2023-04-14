package collector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const (
	exporter_name          = "docker_exporter"
	namespace              = "docker"
	exporter               = "exporter"
	container_subnamespace = "container"
	image_subnamespace     = "image"
)

var DurationBuckets = []float64{.005, .01, .05, .1, .5, 1, 5}

func Name() string {
	return exporter_name
}

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "collector_duration_seconds"),
		"Collector time duration.",
		[]string{"collector"}, nil,
	)
)

type Exporter struct {
	client   *client.Client
	metrics  Metrics
	scrapers []Scraper
}

func New(metrics Metrics, scrapers []Scraper, opts client.Opt) (*Exporter, error) {

	c, err := client.NewClientWithOpts(opts)
	if err != nil {
		return nil, fmt.Errorf("create docker client err: %s", err)
	}

	return &Exporter{
		client:   c,
		metrics:  metrics,
		scrapers: scrapers,
	}, nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.metrics.TotalScrapes.Desc()
	ch <- e.metrics.Error.Desc()
	e.metrics.ScrapeErrors.Describe(ch)
	ch <- e.metrics.DockerUp.Desc()
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	e.scrape(ctx, ch)
	ch <- e.metrics.TotalScrapes
	ch <- e.metrics.Error
	e.metrics.ScrapeErrors.Collect(ch)
	ch <- e.metrics.DockerUp
	ch <- e.metrics.SuccessfulProbeTime
}

func (e *Exporter) scrape(ctx context.Context, ch chan<- prometheus.Metric) {

	e.metrics.TotalScrapes.Inc()

	scrapeTime := time.Now()

	if _, err := e.client.Ping(ctx); err != nil {
		log.Error(err)
		e.metrics.DockerUp.Set(0)
		e.metrics.Error.Set(1)
		return
	}

	e.metrics.DockerUp.Set(1)
	e.metrics.Error.Set(0)

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "ping")

	var wg sync.WaitGroup
	defer wg.Wait()
	for _, scraper := range e.scrapers {
		wg.Add(1)
		go func(scraper Scraper) {
			defer wg.Done()
			label := scraper.Name()
			scrapeTime := time.Now()
			if err := scraper.Scrape(ctx, e.client, ch); err != nil {
				log.WithField("scraper", scraper.Name()).Error(err)
				e.metrics.ScrapeErrors.WithLabelValues(label).Inc()
				e.metrics.Error.Set(1)
			}
			ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), label)
		}(scraper)
	}
	e.metrics.SuccessfulProbeTime.SetToCurrentTime()
}

type Metrics struct {
	TotalScrapes        prometheus.Counter
	ScrapeErrors        *prometheus.CounterVec
	Error               prometheus.Gauge
	DockerUp            prometheus.Gauge
	SuccessfulProbeTime prometheus.Gauge
}

func NewMetrics() Metrics {
	subsystem := exporter
	return Metrics{
		TotalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "scrapes_total",
			Help:      "Total number of times docker was scraped for metrics.",
		}),
		ScrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occurred scraping a docker.",
		}, []string{"collector"}),
		Error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from docker resulted in an error (1 for error, 0 for success).",
		}),
		DockerUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the docker is up.",
		}),
		SuccessfulProbeTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "probe_successfully_completed_time",
			Help:      "When the last Docker probe was successfully completed.",
		}),
	}
}
