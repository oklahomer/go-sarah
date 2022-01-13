#!/bin/bash

# go version go1.17.5 darwin/arm64 -> 1.17
GO_MINOR_VERSION=$(go version | grep -o 'go[0-9]\+\.[0-9]\+\(\[0-9]\+\)\?' | tr -d 'go')

if [ "$GO_MINOR_VERSION" = "1.15" ] ; then
    # "go get" and "go install" works, but version suffix is not supported yet.
    eval "go install github.com/mattn/goveralls"
else
    # 1.16 or later version supports "go install" with version suffix.
    # See https://tip.golang.org/doc/go1.16#tools
    # "go install now accepts arguments with version suffixes (for example, go install example.com/cmd@v1.0.0).
    # This causes go install to build and install packages in module-aware mode, ignoring the go.mod file in the
    # current directory or any parent directory, if there is one.
    #
    # 1.17 now recommends the use of "go install" instead of "go get" to install commands outside the main module.
    # See https://go.dev/doc/go1.17#go-get
    # "go get prints a deprecation warning when installing commands outside the main module (without the -d flag).
    # go install cmd@version should be used instead to install a command at a specific version, using a suffix like
    # @latest or @v1.2.3. In Go 1.18, the -d flag will always be enabled, and go get will only be used to change
    # dependencies in go.mod."
    #
    # 1.18 uses "go install" to install the latest version of an executable outside the context of the current module.
    # See https://tip.golang.org/doc/go1.18#tools
    # "To install the latest version of an executable outside the context of the current module, use go install
    # example.com/cmd@latest. Any version query may be used instead of latest."
    eval "go install github.com/mattn/goveralls@latest"
fi
