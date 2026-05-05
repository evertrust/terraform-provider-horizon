# Trust chain data source scenarios. The child module enrolls a centralized
# certificate then queries the trust chain in every supported order, so we can
# assert the chain is well-formed and that the leaf/root ordering switches as
# requested.

variable "endpoint" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
}
variable "centralized_profile" { type = string }

run "trust_chain_default_and_orders" {
  command = apply

  module {
    source = "./modules/trust_chain"
  }

  variables {
    endpoint = var.endpoint
    username = var.username
    password = var.password
    profile  = var.centralized_profile
    cn       = "trust-chain.tf-test.internal"
  }

  # --- default order is leaf_to_root ---
  assert {
    condition     = data.horizon_certificate_trust_chain.default_order.id != ""
    error_message = "default_order id must be set"
  }
  assert {
    condition     = data.horizon_certificate_trust_chain.default_order.order == "leaf_to_root"
    error_message = "default order must be leaf_to_root"
  }
  assert {
    condition     = length(data.horizon_certificate_trust_chain.default_order.chain) >= 2
    error_message = "trust chain must contain at least the leaf and one issuer"
  }
  assert {
    condition = (
      data.horizon_certificate_trust_chain.default_order.length ==
      length(data.horizon_certificate_trust_chain.default_order.chain)
    )
    error_message = "length attribute must match chain entry count"
  }
  assert {
    condition     = data.horizon_certificate_trust_chain.default_order.chain_pem != ""
    error_message = "chain_pem must be set"
  }

  # --- leaf_to_root: first PEM is the leaf we submitted ---
  assert {
    condition = (
      data.horizon_certificate_trust_chain.leaf_to_root.chain[0] ==
      horizon_certificate.test.certificate
    )
    error_message = "leaf_to_root chain[0] must be the submitted leaf certificate"
  }
  assert {
    condition = (
      data.horizon_certificate_trust_chain.leaf_to_root.id ==
      data.horizon_certificate_trust_chain.default_order.id
    )
    error_message = "leaf_to_root id must equal the default order id (same input + order)"
  }

  # --- root_to_leaf: last PEM is the leaf, ordering is reversed ---
  assert {
    condition = (
      data.horizon_certificate_trust_chain.root_to_leaf.chain[
        length(data.horizon_certificate_trust_chain.root_to_leaf.chain) - 1
      ] == horizon_certificate.test.certificate
    )
    error_message = "root_to_leaf last chain entry must be the submitted leaf certificate"
  }
  assert {
    condition = (
      data.horizon_certificate_trust_chain.root_to_leaf.length ==
      data.horizon_certificate_trust_chain.leaf_to_root.length
    )
    error_message = "leaf_to_root and root_to_leaf must contain the same number of certificates"
  }
  assert {
    condition = (
      data.horizon_certificate_trust_chain.root_to_leaf.id !=
      data.horizon_certificate_trust_chain.leaf_to_root.id
    )
    error_message = "different orders must produce different ids"
  }

  # --- issuer_* orders are accepted and return at least one certificate ---
  assert {
    condition     = length(data.horizon_certificate_trust_chain.issuer_leaf_to_root.chain) >= 1
    error_message = "issuer_leaf_to_root must return at least one certificate"
  }
  assert {
    condition     = length(data.horizon_certificate_trust_chain.issuer_root_to_leaf.chain) >= 1
    error_message = "issuer_root_to_leaf must return at least one certificate"
  }
  assert {
    condition = (
      data.horizon_certificate_trust_chain.issuer_leaf_to_root.length ==
      data.horizon_certificate_trust_chain.issuer_root_to_leaf.length
    )
    error_message = "issuer_*_to_* must return the same number of certificates"
  }
}
