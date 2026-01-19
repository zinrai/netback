# Example: Arista cEOS with containerlab

This example demonstrates how to use netback with Arista cEOS running in containerlab.

## Prerequisites

- Docker
- [containerlab](https://containerlab.dev/)
- Arista cEOS image (see [Getting Arista cEOS image](https://containerlab.dev/manual/kinds/ceos/#getting-arista-ceos-image))
- netback binary

## Setup

### 1. Import cEOS image

```bash
$ sudo docker import cEOS64-lab-4.34.4M.tar.xz ceos64:4.34.4M
```

### 2. Deploy containerlab topology

```bash
$ sudo containerlab deploy -t containerlab.yaml
```

This creates a single cEOS node accessible at 172.20.20.2.

### 3. Run netback

```bash
$ netback -model model.yaml -routerdb routerdb.yaml
```

Expected output:

```
2026/01/19 20:04:46 eos-01: connecting...
2026/01/19 20:04:46 eos-01: ssh connected
2026/01/19 20:04:46 eos-01: waiting for prompt...
2026/01/19 20:04:46 eos-01: executing post_login...
2026/01/19 20:04:46 eos-01: executing comments...
2026/01/19 20:04:46 eos-01: executing commands...
2026/01/19 20:04:47 eos-01: ok
2026/01/19 20:04:47 Completed: 1 success, 0 failed
```

### 4. Check output

```bash
$ cat configs/dc-tokyo/eos-01
```

## Cleanup

```bash
$ sudo containerlab destroy -t containerlab.yaml
```
