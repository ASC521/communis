# Communis

***Lotus communis*** is translated from Latin to which means "a general or common place", such as a statement of proveberial wisdom.  Commonplace books are personal notebooks used to compile any information the owner finds interesting or useful.

## Build

1. `./build.sh -b`

### Container

1. `./build.sh -b -c`

## Installation

### Linux User Installation

1. Download linux binary to `~/.local/bin`

2. Generate systemd unit file and start service

```bash
communis generate systemd-unit > ~/.config/systemd/user/communis.service
systemctl --user daemon-reload
systemctl --user enable communis
```

### Docker

1. Pull image

```bash
docker pull ghcr.io/asc521/communis:{TAG_VERSION}
```

#### Docker Compose
```
communis:
	container_name: communis
	image: ghcr.io/asc521/communis:{TAG_VERSION}
	command: serve
	volumes:
		~/.config/communis:/etc/opt/communis
		~/.local/share/communis:/var/opt/communis
	ports:
		6789:6789
	restart:
		unless-stopped
```

### Podman

1. Pull container image
```bash
podman pull ghcr.io/asc521/communis:{TAG_VERSION}
```

2. Create volume to persist application data
```bash
podman volume communis-data
```

3. Run container directory
```bash
podman run \ 
-p 6789:6789 \
-v communis-data:/var/opt/communis \
-v ~/.config/communis:/etc/opt/communis \
ghcr.io/asc521/communis:{TAG_VERSION} serve
```

4. Generate systemd unit file for automated management of application
```bash
mkdir -p ~/.config/containers/systemd
podman run ghcr.io/asc521/communis:{TAG_VERSION} generate systemd-container > ~/.config/containers/systemd/communis.container
systemctl --uesr daemon-reload
systemctl --user enable communis.service
```

## Uninstall
### Linux

1. `sudo communis uninstall`
