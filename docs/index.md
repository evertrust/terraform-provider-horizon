---
layout: ""
page_title: "Provider: Horizon"
description: |-
  The Horizon Provider
---

# Horizon Provider

The Horizon Provider is a cutting-edge Certificate Lifecycle Management platform designed to seamlessly manage,
automate, and orchestrate the entire lifecycle of digital certificates. Leveraging the power of advanced PKI
capabilities, Horizon Provider offers a centralized solution that ensures the secure issuance, renewal, and revocation
of certificates across multiple environments.

## Example Usage of Horizon Provider

The Horizon Provider offers a flexible and secure way to manage your certificate lifecycle within your Terraform
configuration. You can authenticate using either API credentials or a certificate and key pair. Here's how to set up
each method:

### Authentication with API Credentials

In this method, you'll use your x_api_id and x_api_key to authenticate. These credentials are provided by Horizon and
ensure secure access to the platform.

```terraform
provider "horizon" {
  x_api_id  = "example"
  x_api_key = "example"
  endpoint  = "https://horizon.example"
}
```

#### Parameters:

- `x_api_id`(string): Your unique API ID provided by Horizon.
- `x_api_key`(string): Your secure API key.
- `endpoint`(string): The URL of the Horizon service you are connecting to.

### Authentication with a Certificate and Key

Alternatively, you can authenticate using a certificate and key file. This method is particularly useful for
environments that require an extra layer of security.

```terraform
provider "horizon" {
  cert     = "path/to/certificate/file"
  key      = "path/to/key/file"
  endpoint = "https://horizon.example"
}
```

#### Parameters:

- `cert`(string): The path to your certificate file.
- `key` (string): The path to your key file.
- `endpoint` (string): The URL of the Horizon service you are connecting to.
