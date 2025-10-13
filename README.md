# CClient

Fixes TLS and stuff. Uses the yawning utls fork instead of the original refraction-networking one.

# Example

```go
package main

import (
    "log"
    "time"

    tls "github.com/Titanium-ctrl/utls"
    "github.com/Titanium-ctrl/cclient"
)

func main() {
    client, err := cclient.NewClient(
        tls.HelloChrome_Auto,
        nil,          // no proxy
        true,         // allow redirects
        false,        // skip TLS verification
        false,        // force HTTP/1.1
        6*time.Second,
        "",          // TLS key log file
        false,        // enable hardware RX timestamps
        nil,          // connection callback
    )
    if err != nil {
        log.Fatal(err)
    }

    resp, err := client.Get("https://www.google.com/")
    if err != nil {
        log.Fatal(err)
    }
    resp.Body.Close()

    log.Println(resp.Status)
}
```

# Notes

If you experience any issues with the gitlab.com/yawning/utls import during installation, please try: `go get gitlab.com/yawning/utls`. Some path issue with go and gitlab ¯\\\_(ツ)\_/¯

The go.mod issue with git etc. was fixed by using my fork of the project with a change to the go.mod file instead of yawning's fork
