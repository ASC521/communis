# Communis

***Lotus communis*** is translated from Latin to which means "a general or common place", such as a statement of proveberial wisdom.  Commonplace books are personal notebooks used to compile any information the owner finds interesting or useful.

## Build

1. `./build.sh -b`

## Installation
### Linux

1. Download linux binary and save to `/usr/local/bin/`.
2. Create user for application (default username: communis).  `sudo useradd --system --no-create-home communis`
3. As root user execute `sudo communis install`
4. `sudo systemctl enable --now communis`

## Uninstall
### Linux

1. `sudo communis uninstall`
