# Communis

***Lotus communis*** is translated from Latin to which means "a general or common place", such as a statement of proveberial wisdom.  Commonplace books are personal notebooks used to compile any information the owner finds interesting or useful.

## Build

1. `./build.sh -b`

### Container

1. `./build.sh -b -c`

## Installation

### Linux

#### User Level Install

1. Create user to run the application (default username: communis-runner).

```bash
sudo useradd --system --create-home --home-dir /home/podman-communis --shell /sbin/nologin communis-runner
```

2. Enable lingering for service user 

```bash
sudo loginctl enable-linger communis-runner
```

3. Login as newly created user 

```bash
sudo -u communis-runner bash
```

4. Download linux binary and save to `~/.local/bin/communis`

5. Generate a config file 

```bash
communis generate config > ~/.config/communis/config.toml
```

6. Generate a systemd unit file 

```bash
communis generate systemd-unit > ~/.config/systemd/user/communis.service
```

7. Reload systemd daemon

```bash
systemctl --user daemon-reload
```

8. Enable service

```bash
systemctl --user enable --now communis.service
```

#### System Wide Install

1. Create a service user 

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin communis-runner
```

2. Download linux binary and save to `/opt/communis/bin`

3. Set ownership of app files 

```bash
sudo chown -R communis-runner:communis-runner /opt/communis
```

4. Generate a config file 

```bash
sudo /opt/communis/bin/communis -system generate config > /etc/opt/communis/config.toml
```

5. Set ownership of config file 

```bash
sudo chown communis-runner:communis-runner /etc/opt/communis/config.toml
```

6. Generate a systemd unit file 

```bash
sudo /opt/communis/bin/communis -system generate systemd-unit -username communis-runner > /etc/systemd/system/communis.service
```

7. Set ownership of data directory 

```bash
sudo chown -R communis-runner:communis-runner /var/opt/communis
```

7. Reload systemd daemon 

```bash
sudo systemctl daemon-reload
```

8. Enable communis service 

```bash
sudo systemctl enable --now communis.service
```

### Docker

### Podman

#### User Level Install

1. Create application user 

```bash
sudo useradd --system --create-home --home-dir /home/podman-communis --shell /sbin/nologin communis-runner
```

2. Enable lingering for service user 

```bash
sudo loginctl enable-linger communis-runner
```

3. Login as newly created user 

```bash
sudo -u communis-runner bash
```

4. Pull image 

```bash
podman pull localhost/communis:<tag>
```

5. Generate a config file 

```bash
podman -user run localhost/communis generate config > ~/.config/communis/config.toml
```

6. Generate a systemd container file 

```bash
podman -user run localhost/communis generate systemd-container > ~/.config/containers/systemd/communis.container
```

7. Reload systemd daemon 

```bash
systemctl --user daemon-reload
```

8. Start communis 

```bash
systemctl --user start communis.service
```

9. Enable communis to start on login 

```bash
systemctl --user enable communis.service
```

## Uninstall
### Linux

1. `sudo communis uninstall`
