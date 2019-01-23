hmetrics
========

[![Continuous Integration](https://secure.travis-ci.org/PennockTech/hmetrics.svg?branch=master)](http://travis-ci.org/PennockTech/hmetrics)
[![Documentation](https://godoc.org/go.pennock.tech/hmetrics?status.svg)](https://godoc.org/go.pennock.tech/hmetrics)
[![Coverage Status](https://coveralls.io/repos/github/PennockTech/hmetrics/badge.svg)](https://coveralls.io/github/PennockTech/hmetrics)

This is Heroku's Go-specific language metrics, as a standalone package.

Heroku's support is inside an organization-internal base dumping-ground repo,
which pulls in quite a few dependencies and is not a stable interface.

This package reproduces the core functionality of
`github.com/heroku/x/hmetrics` in a more usable API and without all the other
dependencies.

This package uses [semantic versioning](https://semver.org/).

We do not support the silent on-init enabling method of hmetrics: all
production code which might error should log what it's doing and we are
designed to integrate with production logging.

This library does not panic, by policy, even when it probably should.
If the `Spawn()` function returns a non-nil error then that's probably
panic-worthy.

## Usage

```go
import (
    "log"

    "go.pennock.tech/hmetrics"
    )

func main() {
    // This depends upon your logging library, etc.
    msg, cancel, err := hmetrics.Spawn(func(e error) {
        log.Printf("hmetrics error: %s", e)
        })
    if err != nil {
        // if environment variable not found or empty, that's not an error,
        // this is something which means we expected to log but never will
        // be able to.
        panic(err)
    }
    if cancel != nil {
        defer cancel()
    }
    log.Print(msg) // for warm fuzzy feelings that stuff has started correctly

    // do your work
}
```

## Bugs

Probably plenty, starting with a complete lack of tests as this was written in
frustration during one night of coding.

Yes, the absence of any tests is a bug.

Pull requests very welcome, as well as bug reports (unless it's just to say
"there are no tests!").

At the time I write this notice, I haven't even tried using this code yet.
It's very shiny new.  It passes `go build`, `go vet ./...` and `golint ./...`
and that's it.
