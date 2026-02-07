package main

import (
	"os"

	"github.com/fpp-125/metaclaw/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
