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
