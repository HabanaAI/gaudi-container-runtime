# Habana-Container-Runtime

A wrapper for [runc](https://github.com/opencontainers/runc) that configures Habana devices and their network interfaces using `habana-container-hook` and `habana-container-cli`.
By default, environment variables select only accelerators already authorized by the container engine's OCI device entries and cgroup permissions. Environment variables alone do not authorize device access.

- [Habana-Container-Runtime](#habana-container-runtime)
  - [Installation](#installation)
    - [Build from source](#build-from-source)
      - [Build binaries](#build-binaries)
      - [Build package from source](#build-package-from-source)
        - [Debian package](#debian-package)
        - [RPM package](#rpm-package)
    - [Install pre-built package](#install-pre-built-package)
      - [Ubuntu distributions](#ubuntu-distributions)
      - [CentOS and Amazon linux distributions](#centos-and-amazon-linux-distributions)
  - [Docker Engine setup](#docker-engine-setup)
      - [Daemon configuration file](#daemon-configuration-file)
  - [ContainerD Setup](#containerd-setup)
    - [Containerd configuration file](#containerd-configuration-file)
  - [CRI-O Setup](#cri-o-setup)
    - [CRI-O configuration file](#cri-o-configuration-file)
  - [Usage example](#usage-example)
  - [Environment variables (OCI spec)](#environment-variables-oci-spec)
    - [`HABANA_VISIBLE_DEVICES`](#habana_visible_devices)
    - [`HABANA_VISIBLE_MODULES`](#habana_visible_modules)
    - [`HABANA_RUNTIME_ERROR` **Auto generated**](#habana_runtime_error-auto-generated)
  - [Config](#config)
  - [Issues and Contributing](#issues-and-contributing)

## Installation

### Build from source

All binaries are build under dist/{BINARY_NAME}_architecture/{BINARY_NAME}.

Available architectures:
- linux_amd64
- linux_386
- linux_arm64

#### Build binaries

```bash
# Build all binaries
make build-binary

# Build only habana-container-runtime
make build-runtime

# Build only habana-container-hook
make build-hook

# Build only habana-container-cli (libhabana-container)
make build-cli
```

After building the binaries, copy the config from `packaging/config.toml`
into `/etc/habana-container-runtime/config.toml` and edit for your
environment.

#### Build package from source

You must have docker installed. Building the packages is done using goreleaser.

```bash
make release
```

Artifacts are found under `dist/` folder.

### Install pre-built package

#### Ubuntu distributions

1. download package from https://vault.habana.ai/artifactory/debian/<distribution>/pool/main/h/habanacontainerruntime/habanalabs-container-runtime-<RELEASE>.amd64.deb
2. Install the `habana-container-runtime.deb` package:
```
sudo dpkg -i habana-container-runtime.deb
```

#### CentOS and Amazon linux distributions

1. download package from https://vault.habana.ai/artifactory/centos/<major>/<version>/<binary_arch>.rpm
2. Install the `habana-container-runtime.rpm` package:

```bash
sudo yum install habana-container-runtime.rpm
```

To register the `habana` runtime, use the method below that is best suited
to your environment. You might need to merge the new argument with your
existing configuration.

## Docker Engine setup

#### Daemon configuration file

```bash
sudo tee /etc/docker/daemon.json <<EOF
{
    "runtimes": {
        "habana": {
            "path": "/usr/bin/habana-container-runtime",
            "runtimeArgs": []
        }
    }
}
EOF
sudo systemctl restart docker
```

You can optionally reconfigure the default runtime by adding the following to `/etc/docker/daemon.json`:
```
"default-runtime": "habana"
```

## ContainerD Setup

### Containerd configuration file

```bash
sudo tee /etc/containerd/config.toml <<EOF
disabled_plugins = []
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "habana"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.habana]
          runtime_type = "io.containerd.runc.v2"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.habana.options]
            BinaryName = "/usr/bin/habana-container-runtime"
  [plugins."io.containerd.runtime.v1.linux"]
    runtime = "habana-container-runtime"
EOF
sudo systemctl restart containerd
```

## CRI-O Setup

Create new config file at `/etc/crio/crio.conf.d/99-habana-ai.conf`.

### CRI-O configuration file

```toml
[crio.runtime]
default_runtime = "habana-ai"

[crio.runtime.runtimes.habana-ai]
runtime_path = "/usr/local/habana/bin/habana-container-runtime"
monitor_env = [
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
]
```

Restart crio service: `systemctl restart crio.service`

## Usage example

Currently habana-container-runtime has to be used with habana-container-hook and libhabana-container
Authorize accelerator 0 through Docker's device options. `HABANA_VISIBLE_DEVICES=all`
then selects only that authorized accelerator, not every accelerator on the host.

```bash
docker run --rm --runtime=habana \
  --device=/dev/accel/accel0 --device=/dev/accel/accel_controlD0 \
  -e HABANA_VISIBLE_DEVICES=all ubuntu:22.04 /bin/bash -c "ls /dev/accel/*"

/dev/accel/accel0
/dev/accel/accel_controlD0
```


## Environment variables (OCI spec)

Each environment variable maps to an command-line argument for `habana-container-cli` from [libhabana-container](https://github.com/HabanaAI/libhabana-container).

### `HABANA_VISIBLE_DEVICES`
This variable controls which Habana devices will be made accessible inside the container.

#### Possible values
* `0,1,2` …: a comma-separated list of index(es).
* `all`: select all already-authorized Habana accelerators. This is the default value in our container images.
* Empty, `none`, or `void`: perform no additional Habana setup.

Without a selector, no additional setup occurs unless `visible_devices_all_as_default`
is enabled. That option is also bounded by the authorized device set.
Explicit IDs outside the allocation, duplicate selector keys, malformed selectors,
and conflicting device/module/PCI selections fail container creation.


### `HABANA_VISIBLE_MODULES` **Auto generated**
The variable holds the requested accelerators module ids.
Order does not overlap with the HABANA_VISIBLE_DEVICES order.

### `HABANA_RUNTIME_ERROR` **Auto generated**
Variable hold the last error from the runtime flow. The runtime
does not fail the pod creation in most cases, so we propagate the error inside the container for debugging purposes.


## Config

See options [here](./packaging/config.toml)

The default allocation-only policy applies to OCI and legacy modes. It matches
host accelerator character-device major/minor identities against the original
OCI device list and requires effective read/write cgroup access. Accelerator
permissions are preserved, not upgraded. Associated network interfaces and,
when `mount_uverbs=true`, RDMA devices are selected by the allocated accelerator's
PCI identity. `mount_accelerators=false` alone is not an authorization policy.

This policy trusts the container engine to construct device entries. Kubernetes
admission must prevent ordinary workloads from injecting arbitrary host devices;
neither environment values, pod annotations, container UID 0, nor capabilities
establish an allocation. CDI mode leaves OCI modification to the CDI integration.

For trusted standalone installations that intentionally permit environment-only
device grants, a host administrator can opt in at the top level of
`/etc/habana-container-runtime/config.toml`:

```toml
allow-env-device-requests = true
```

This compatibility option restores environment-driven host device selection and
must not be enabled for an untrusted shared Kubernetes runtime. The older
`accept-habana-visible-devices-envvar-when-unprivileged` option does not enable it.
Upgrade the runtime, hook, and CLI together. Existing environment-only Docker
commands must either explicitly grant devices as above or use this trusted
standalone opt-in. Prefer a fully configured CDI deployment for Kubernetes.

## Issues and Contributing

* Please let us know by [filing a new issue](https://github.com/HabanaAI/habana-container-runtime/issues/new)
* You can contribute by opening a [pull request](https://docs.github.com/en/github/collaborating-with-issues-and-pull-requests/about-pull-requests)
