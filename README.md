# Terraform Provider: Horizon

The Horizon Provider allows [Terraform](https://terraform.io) to manage [Horizon](https://evertrust.fr/horizon)
certificates and their lifecycle.

## Quick Start

* [Provider Documentation](https://registry.terraform.io/providers/EverTrust/horizon/latest)
* [Horizon Documentation](https://docs.evertrust.fr/horizon/2.8/)
* [Evertrust Website](https://evertrust.io)

## Compatibility

Compatibility between the provider and Horizon versions is as follows:

| Provider version | Horizon version |
|:----------------:|:---------------:|
|    `>= 0.1.x`    |     `<=2.3`     |
|    `>= 0.2.x`    |     `>=2.4`     |

## Development

### Requirements

* [Go](https://golang.org/doc/install) >= 1.21
* [Terraform](https://www.terraform.io/downloads.html) >= 0.13
* An Horizon instance

### Building The Provider

To build and test the provider, follow the steps
described [here](https://developer.hashicorp.com/terraform/plugin/debugging). You have two options:

- Use Development Overrides if you need to quickly test out a change to the provider for a single configuration. This is
  the fastest way to test a change.
- Use Debugger-based Debugging which will allow you to run the provider separately from Terraform and attach a debugger
  to it. This is the best way to test a change thoroughly.

## Releasing

The release process is automated via GitHub Actions, which can be audited int
the [release.yml](./.github/workflows/release.yml) file.

To release a new version, you need to create a new tag. The tag name must follow
the [Semantic Versioning](https://semver.org/) convention, starting with the letter `v`.

## License

[GNU GPL v3](./LICENSE)
