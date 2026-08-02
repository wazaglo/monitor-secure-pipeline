# Runbook: Disk Full

Alert: **DiskSpaceRunningOut** (critical) — `/` free space below 10%.

## Symptoms

- Alert fires with `mountpoint="/"`
- Services start writing and then fail
- Prometheus/Loki/Tempo data volumes grow

## Steps

### 1. Confirm the free space

```bash
df -h /
```

### 2. Find the largest consumers

```bash
docker system df
sudo du -sh /var/lib/docker/volumes 2>/dev/null | sort -h | tail
```

Named volumes likely to grow: `prometheus-data`, `loki-data`, `tempo-data`,
`grafana-data`.

### 3. Prune safely

Remove unused build cache and dangling images:

```bash
docker system prune -af --volumes
```

> `--volumes` deletes all unused named volumes — data loss risk. Prefer targeted
> removal shown next when you need the data.

### 4. Trim telemetry data (targeted)

```bash
docker compose stop prometheus loki tempo
docker volume rm monitor-secure-pipeline_prometheus-data \
                monitor-secure-pipeline_loki-data \
                monitor-secure-pipeline_tempo-data
docker compose up -d prometheus loki tempo
```

This resets metrics/logs/traces history. Do it only when the data is
non-essential.

### 5. Long-term prevention

- Loki has `retention_period: 744h` and compactor retention enabled in
  `monitoring/loki/loki-config.yml`.
- Tempo has `block_retention: 48h` in `monitoring/tempo/tempo-config.yml`.
- On EC2, increase `root_volume_size` in `terraform/variables.tf` and
  re-apply.
