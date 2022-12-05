# horizon-provider-terraform

The Horizon Provider allows [Terraform](https://terraform.io) to manage [Horizon](https://evertrust.fr/horizon) resources.

* [Evertrust website](https://evertrust.fr)

## Quick Starts

* [Provider Documentation](https://github.com/EverTrust/terraform-provider-horizon/tree/main/docs)

## Provider Usage

The Horizon Provider allow you to manage the life cycle of a certificate. From the creation to the revocation.

### Upgrading the provider 

The Horizon Provider doesn't upgrade automatically once you've started using it. After a new release you can run 
```bash
make init
```
to upgrade to the latest stable version of the Horizon Provider.

### Upgrading horizon-go

The Go SDK horizon-go may be updated and cause some errors, or will not allow you to exploit some features. In that case youcan run 
```bash
make update
```
to upgrade to the latest stable version of horizon-go.