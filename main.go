package main

import (
	"context"
	"flag"
	"log"

	"github.com/anorph/terraform-provider-foundrydb/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "enable provider debug mode")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/anorph/foundrydb",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err)
	}
}
