# ics

> https://github.com/PuloV/ics-golang 在此基础上进行的修改


# quick start

## Download and install
```shell
go get github.com/songzhibin97/ics
```

## import
```go
import "github.com/songzhibin97/ics"
```

## use
```go
package main

import "github.com/songzhibin97/ics"

func main() {
	// parser,err := ics.NewParserByUrl(url)
	// parser,err := ics.NewParserByFile(file)
	parser, err := ics.NewParserByContent(content)
	if err != nil {
		panic(err)
	}
}

```

