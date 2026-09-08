package main

import (
	"os"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
	"github.com/ManuelReschke/fotoly-cli/internal/cli"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	a := cli.New(brand.PixelFox)
	a.Version = version
	a.Commit = commit
	return cli.Run(a)
}
