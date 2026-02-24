#!/usr/bin/env bash

function get_os() {
  unameOut="$(uname -s)"
  case "${unameOut}" in
  Linux*)
    echo -n "linux"
    ;;
  Darwin*)
    echo -n "macos"
    ;;
  CYGWIN*)
    echo -n "cygwin"
    ;;
  MINGW*)
    echo -n "mingw"
    ;;
  *)
    echo "Cannot detect your operating system.  Exiting."
    exit 1
    ;;
  esac

}

os="$(get_os)"

function check_available() {
  which $1 >/dev/null
  if [ $? -ne 0 ]; then
    echo "**** ERROR needed program missing: $1"
    exit 1
  fi
}

check_available 'which'
check_available 'realpath'
check_available 'dirname'

bashver="${BASH_VERSION:0:1}"

if ! [[ "$bashver" =~ ^[5-9]$ ]]; then
  echo 'You need MOAR bash-fu!  Your version of `bash` is waaaay too old!  What are you running?  Commodore 64?'
  echo "Your bash version is: ${BASH_VERSION}"
  echo "Your need at least a bash version of 5 or higher"
  echo
  case "${os}" in
  linux)
    echo "Thank God, you're running Linux.  There's hope."
    echo "Use your package manager to upgrade bash."
    echo "If your Linux distribution can't get a recent version of bash, change distros."
    ;;
  macos)
    echo 'MacOS: likely `brew install bash` will be your friend.'
    ;;
  cygwin)
    echo "Uhhhh ... CygWin.  Not sure how to help here."
    ;;
  mingw)
    echo "Uhhhh ... MinGW.  Not sure how to help here."
    ;;
  *)
    echo "I have no idea."
    echo "Repent sins."
    ;;
  esac
  exit 1
fi

script_dir=$(dirname "$(realpath "$0")")
cwd="$(echo "$(pwd)")"
function cleanup() {
  cd "$cwd"
}

# Make sure that we get the user back to where they started
trap cleanup EXIT

# This is necessary because we reference things relative to the script directory
cd "$script_dir"


function usage() {
    echo "Usage: build.sh [-h|--help] [-b|--build]"
    echo
    echo "    Build communis application"
    echo
    echo "  -h|--help                  This help text"
    echo "  -b|--build                 Build communis application"
    echo "  -t|--test                  Run unit tests"
    echo "  -r|--run                   Run web application"
    echo
}

build=0
true=0
run=0

case "$os" in
    linux)
	default_target="linux"
	;;
    macos)
	default_target="darwin"
	;;
    cygwin | mingw)
	default_target="windows"
	;;
esac

while [[ $# -gt 0 ]]; do
    key="$1"

    case $key in
	-h | --help)
	    usage
	    exit 0
	    ;;
	-b | --build)
	    build=true
	    shift
	    ;;
	-t | --test)
	    test=true
	    shift
	    ;;
	-r | --run)
	    run=true
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

if [ "$test" = true ]; then
    echo "Running unit tests"
    go test -v ./...
    exit 0
fi


if [ "$build" = true ]; then
    echo "Building communis with local go: $(which go)"
    go build -v -o ./dist/exec/communis ./cmd/cli/
    exit 0
fi

if [ "$run" = true ]; then
    echo "Running communis web application"
    go run ./cmd/cli web run -debug
    exit 0
fi
