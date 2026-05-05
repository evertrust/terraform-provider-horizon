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
  description = "How many days before not_after the provider should consider the cert in its renewal window. With the CA issuing certs for roughly one year, any value >= 400 makes any refresh after enroll fall inside the window."
}
