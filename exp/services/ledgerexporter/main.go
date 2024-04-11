package main

import exporter "github.com/hcnet/go/exp/services/ledgerexporter/internal"

func main() {
	app := exporter.NewApp()
	app.Run()
}
