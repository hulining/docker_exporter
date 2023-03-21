package collector

import (
	"strings"

	"github.com/docker/docker/api/types"
)

func getNameByInspect(container types.ContainerJSON) string {
	if container.Name != "" {
		return strings.Trim(container.Name, "/")
	}
	return container.ID[:12]
}

func getNameByStatus(stats types.StatsJSON) string {
	if stats.Name != "" {
		return strings.Trim(stats.Name, "/")
	}
	return stats.ID[:12]
}
