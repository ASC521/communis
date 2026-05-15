#!/bin/bash
set -euo pipefail

function check_available() {
  local path
  if ! path=$(command -v "$1"); then
    echo "**** ERROR needed program missing: $1" >&2
    exit 1
  fi
  echo "$path"
}

function pull_container_image() {
    local container_app="$1"
    local run_as="$2"

    LATEST_TAG=$(
	curl -s \
	     "https://api.github.com/orgs/ASC521/packages/container/communis/versions" \
	    | jq -r '.[].metadata.container.tags[]?' \
	    | grep -E '^v?[0-9]+\.[0-9]+\.[0-9]+$' \
	    | sort -V \
	    | tail -n1
	      )

    echo "Pulling container image as $run_as..."
    sudo -u "$run_as" "$container_app" pull "ghcr.io/ASC521/communis:$LATEST_TAG" || {
	echo "**** ERROR failed to pull image: $image" >&2
	exit 1
    }
}

function check_container_image() {
  local container_app="$1"
  local image="$2"
  "$container_app" image exists "$image" || {
    echo "**** ERROR podman image not found: $image" >&2
    echo "     Run 'podman pull $image' first" >&2
    exit 1
  }
  echo "$image"
}

function usage() {
    echo "Usage: install.sh [-h|--help] [-lu|--linux-user] [-ls|--linux-system] [-p|--podman]"
    echo
    echo "    Install communis application"
    echo
    echo "  -h |--help                  This help text"
    echo "  -lu|--linux-user            Install communis application as a linux user application"
    echo "  -ls|--linux-system          Install communis application as a linux system application"
    echo "  -p |--podman                Install communis application as a podman container managed by systemd"
    echo
}

linux_user=0
linux_system=0
podman_install=0

while [[ $# -gt 0 ]]; do
    key="$1"

    case $key in
	-h | --help)
	    usage
	    exit 0
	    ;;
	-lu | --linux-user)
	    linux_user=1
	    shift
	    ;;
	-ls | --linux-system)
	    linux_system=1
	    shift
	    ;;
	-p | --podman)
	    podman_install=1
	    shift
	    ;;
	*)
	    echo "ERROR: unknown argument $1"
	    echo
	    usage
	    exit 1
	    ;;
    esac
done

flag_count=$((linux_user + linux_system + podman_install))
if [ "$flag_count" -ne 1 ]; then
    echo "ERROR: exactly one install mode must be specified"
    usage; exit 1
fi


if [ $linux_user = 1 ]; then
    COMMUNIS=$(check_available 'communis')
    echo "Installing communis as a linux user application..."

    echo "Creating service user communis-runner..."
    sudo useradd --system --create-home --home-dir /home/podman-communis --shell /usr/sbin/nologin communis-runner
    sudo loginctl enable-linger communis-runner

    echo "Generating config file @ ~/.config/communis/config.toml..."
    echo "Generating systemd service file @ ~/.config/systemd/user/communis.service..."
    sudo -u communis-runner bash -c "
      	 export XDG_RUNTIME_DIR=/run/user/\$(id -u)
	 mkdir -p \$HOME/.config/communis
	 mkdir -p \$HOME/.config/systemd/user
    	 $COMMUNIS generate config > \$HOME/.config/communis/config.toml
	 $COMMUNIS generate systemd-unit > \$HOME/.config/systemd/user/communis.service
	 systemctl --user daemon-reload
	 systemctl --user enable --now communis.service
"
    echo "Installation complete"
    exit 0
fi


if [ $linux_system = 1 ]; then
    COMMUNIS=$(check_available 'communis')
    echo "Installing communis as a linux system application..."

    echo "Create service user communis-runner..."
    sudo useradd --system --no-create-home --shell /usr/sbin/nologin communis-runner
    
    echo "Setting ownership of application files..."
    sudo chown -R communis-runner:communis-runner /opt/communis

    echo "Generating a config file @ /etc/opt/communis/config.toml..."
    sudo bash -c "$COMMUNIS -system generate config > /etc/opt/communis/config.toml"
    sudo chown communis-runner:communis-runner /etc/opt/communis/config.toml

    echo "Generating systemd unit file..."
    sudo bash -c "$COMMUNIS -system generate systemd-unit -username communis-runner > /etc/systemd/system/communis.service"

    echo "Setting ownership of application data directory..."
    sudo chown -R communis-runner:communis-runner /var/opt/communis

    echo "Reloading systemd and enabling service"
    sudo systemctl daemon-reload
    sudo systemctl enable --now communis.service

    echo "Installation complete"
    
    exit 0
fi

if [ $podman_install = 1 ]; then
    PODMAN_APP=$(check_available 'podman')

    echo "Installing communis as a podman container managed by systemd..."

    echo "Creating service user..."
    sudo useradd --system --create-home --home-dir /home/podman-communis --shell /usr/sbin/nologin communis-runner
    sudo loginctl enable-linger communis-runner

    pull_container_image "$PODMAN_APP" 'communis-runner'
    
    sudo -u communis-runner bash -c "
      	 export XDG_RUNTIME_DIR=/run/user/\$(id -u)
	 mkdir -p \$HOME/.config/communis
	 mkdir -p \$HOME/.config/containers/systemd
    	 $PODMAN_APP run $COMMUNIS_IMAGE generate config > \$HOME/.config/communis/config.toml
	 $PODMAN_APP run $COMMUNIS_IMAGE generate systemd-container > \$HOME/.config/containers/systemd/communis.container
	 systemctl --user daemon-reload
	 systemctl --user enable --now communis.service	 
"
    echo "Installation complete"
    
    exit 0
fi
    
