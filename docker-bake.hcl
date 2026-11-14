variable "GO_VERSION" {
  default = null
}

variable "DESTDIR" {
  default = "./bin"
}

target "_common" {
  args = {
    GO_VERSION = GO_VERSION
  }
}

group "default" {
  targets = ["test"]
}

group "validate" {
  targets = ["lint", "test"]
}

target "test" {
  inherits = ["_common"]
  target = "test"
  output = ["type=cacheonly"]
}

target "lint" {
  inherits = ["_common"]
  target = "lint"
  output = ["type=cacheonly"]
}

target "binary" {
  inherits = ["_common"]
  target = "binary"
  output = ["${DESTDIR}/build"]
}

target "image" {
  inherits = ["_common"]
  target = "image"
  tags = ["gopls-analyzers:local"]
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
}
