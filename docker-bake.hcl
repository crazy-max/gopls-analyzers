variable "GO_VERSION" {
  default = null
}

variable "DESTDIR" {
  default = "./bin"
}

target "_common" {
  args = {
    GO_VERSION = GO_VERSION
    BUILDKIT_CONTEXT_KEEP_GIT_DIR = 1
  }
}

target "docker-metadata-action" {
  tags = ["gopls-analyzers:local"]
}

group "default" {
  targets = ["test"]
}

group "validate" {
  targets = ["lint", "update-validate"]
}

target "test" {
  inherits = ["_common"]
  target = "test-coverage"
  output = ["${DESTDIR}/coverage"]
}

target "lint" {
  inherits = ["_common"]
  target = "lint"
  output = ["type=cacheonly"]
}

group "update" {
  targets = ["update-modules", "update-gopls", "update-xtools"]
}

target "update-modules" {
  inherits = ["_common"]
  target = "update-modules"
  output = ["."]
}

target "update-gopls" {
  inherits = ["_common"]
  target = "update-gopls"
  output = ["type=local,dest=internal/gopls,mode=delete"]
}

target "update-xtools" {
  inherits = ["_common"]
  target = "update-xtools"
  output = ["type=local,dest=internal/xtools,mode=delete"]
}

target "update-validate" {
  inherits = ["_common"]
  target = "update-validate"
  output = ["type=cacheonly"]
}

target "binary" {
  inherits = ["_common"]
  target = "binary"
  output = ["${DESTDIR}/build"]
}

target "binary-all" {
  inherits = ["binary"]
  platforms = [
    "darwin/amd64",
    "darwin/arm64",
    "linux/amd64",
    "linux/arm/v6",
    "linux/arm/v7",
    "linux/arm64",
    "linux/ppc64le",
    "linux/riscv64",
    "linux/s390x",
    "windows/amd64",
    "windows/arm64"
  ]
}

target "artifact" {
  inherits = ["_common"]
  target = "artifact"
  output = ["type=local,dest=${DESTDIR}/artifact,platform-split=false"]
}

target "artifact-all" {
  inherits = ["binary-all", "artifact"]
}

target "image" {
  inherits = ["_common", "docker-metadata-action"]
  target = "image"
}

target "image-all" {
  inherits = ["image"]
  platforms = [
    "linux/amd64",
    "linux/arm/v6",
    "linux/arm/v7",
    "linux/arm64",
    "linux/ppc64le",
    "linux/riscv64",
    "linux/s390x"
  ]
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
}
