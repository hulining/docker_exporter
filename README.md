
# docker_exporter

Exports metrics about a Docker installation. Using golang and reference https://github.com/prometheus-net/docker_exporter

## Usage

```
docker run -d \
   --name docker_exporter \
  --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -p 9417:9417 \
  hulining/dockcer_exporter:${version}
```

## Reference

- Some metrics are referred to [prometheus-net/docker_exporter](https://github.com/prometheus-net/docker_exporter)
- Part of the code and build process is referenced [zhangguanzhang/harbor_exporter](https://github.com/zhangguanzhang/harbor_exporter)
