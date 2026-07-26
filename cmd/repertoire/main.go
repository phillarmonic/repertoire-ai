package main

import (
	"os"

	"github.com/phillarmonic/repertoire-ai/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Execute(version))
}
