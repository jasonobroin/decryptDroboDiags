// version.go
package main

// Access debug capabilities
import _ "net/http/pprof"
import _ "expvar" // access at /debug/vars

const (
	versionString = "Version 6.3.2"
)
