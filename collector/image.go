package collector

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
)

var image_labels = []string{"id", "repo", "tag"}

var (
	image_size = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, image_subnamespace, "size"),
		"Docker image size.",
		image_labels, nil,
	)
)

type ImageScraper struct{}

// Name of the Scraper. Should be unique.
func (s *ImageScraper) Name() string {
	return "image"
}

// Help describes the role of the Scraper.
func (s *ImageScraper) Help() string {
	return "Collect the image info"
}

func (s *ImageScraper) Scrape(ctx context.Context, client *client.Client, ch chan<- prometheus.Metric) error {

	images, err := client.ImageList(ctx, types.ImageListOptions{All: true})
	if err != nil {
		return err
	}
	for _, image := range images {
		id := strings.Split(image.ID, ":")[1][:12]
		imageName := image.RepoTags[0]
		repoTag := strings.Split(imageName, ":")
		ch <- prometheus.MustNewConstMetric(image_size, prometheus.GaugeValue, float64(image.Size), id, repoTag[0], repoTag[1])
	}
	return err
}

var _ Scraper = &ImageScraper{}
