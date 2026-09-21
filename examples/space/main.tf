provider "confluence" {
  site     = "api.atlassian.com"
  cloud_id = "xxxx-xxxx-xxxx-xxxx"
  user     = var.user
  token    = var.token
}
resource "confluence_space" "example" {
  key  = var.space
  name = "Terraformed Space"
}
resource "confluence_content" "example" {
  space = confluence_space.example.key
  title = "Terraformed Page"
  body  = "Terraformed Content"
}

variable "site" {
  type = string
}

variable "user" {
  type = string
}

variable "token" {
  type = string
}

variable "space" {
  type = string
}
output "example" {
  value = confluence_space.example
}
