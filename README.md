# horizon-provider-terraform

The Horizon Provider allows [Terraform](https://terraform.io) to manage [Horizon](https://evertrust.fr/horizon) resources.

* [Evertrust website](https://evertrust.fr)

## Quick Starts

* [Provider Documentation](https://github.com/EverTrust/terraform-provider-horizon/tree/main/docs)

## Provider Usage

The Horizon Provider allow you to manage the life cycle of a certificate. From the création to the revocation.

### Upgrading the provider 

The Horizon Provider doesn't upgrade automatically once you've started using it. After a new release you can run 
```bash
terraform init -upgrade
```
to upgrade to the latest stable version of the Horizon Provider.

