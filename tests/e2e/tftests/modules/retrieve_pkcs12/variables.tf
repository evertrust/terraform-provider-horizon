variable "endpoint" {
  type = string
}

variable "username" {
  type = string
}

variable "password" {
  type      = string
  sensitive = true
}

variable "profile" {
  type = string
}

variable "cn" {
  type = string
}

variable "renew_before_days" {
  type        = number
  default     = null
  description = "Set to >= 400 to force a WebRA renewal on the next apply (the CA issues ~1 year certs)."
}

variable "skip_escrow_check" {
  type    = bool
  default = null
}

variable "expected_source" {
  type        = string
  description = "Expected `source` of the ephemeral resource: which Horizon request must serve the PKCS#12 (enroll_request, renew_request, ...)."
}
