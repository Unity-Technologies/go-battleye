# BattlEye [![Go Report Card](https://goreportcard.com/badge/github.com/multiplay/go-battleye)](https://goreportcard.com/report/github.com/multiplay/go-battleye) [![License](https://img.shields.io/badge/license-BSD-blue.svg)](https://github.com/multiplay/go-battleye/blob/master/LICENSE) [![GoDoc](https://godoc.org/github.com/multiplay/go-battleye?status.svg)](https://godoc.org/github.com/multiplay/go-battleye) [![Build Status](https://travis-ci.org/multiplay/go-battleye.svg?branch=master)](https://travis-ci.org/multiplay/go-battleye)

go-battleye is a [Go](http://golang.org/) client for the [BattlEye RCON Protocol](https://www.battleye.com/downloads/BERConProtocol.txt).


Features
--------
* Full [BattlEye RCON](https://www.battleye.com/downloads/BERConProtocol.txt) support.
* Multi-packet response support.
* Auto keep-alive support.


Installation
------------
```sh
go get -u github.com/multiplay/go-battleye
```


Examples
--------
Using go-battleye is simple, just create a client then execute commands e.g.

```go
package main

import (
	"log"

	battleye "github.com/multiplay/go-battleye"
)

func main() {
	c, err := battleye.NewClient("192.168.1.102:2301", "mypass")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	resp, err := c.Exec("version")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("server version:", resp)
}
```

Server broadcast messages arrive in a buffered read-only channel. By default, there is room for
100 messages. You can get more by using the `MessageBuffer` option in the client constructor:

```go
battleye.NewClient("192.168.1.102:2301", "mypass", battleye.MessageBuffer(500))
```

If the channel gets full, new messages will be dropped. It is your responsibility to drain the channel, e.g.:

```go
func LogServerMessages(c *battleye.Client) {
	for {
		switch {
		case msg := <-c.Messages():
			log.Println(msg)

		// Another case to exit the for loop.
		// ...
		}
	}
}
```

Run integration test using your own BattlEye server:

```
go test -v -tags=integration -run=Integration -address=<BattlEye server address> -password=<admin password>
```


Documentation
-------------
- [GoDoc API Reference](http://godoc.org/github.com/multiplay/go-battleye).


License
-------
go-battleye is available under the [BSD 2-Clause License](https://opensource.org/licenses/BSD-2-Clause).
