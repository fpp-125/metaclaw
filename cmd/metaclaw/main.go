package main

import (
	"os"

	"github.com/metaclaw/metaclaw/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
