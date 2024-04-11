package main

import exporter "github.com/shantanu-hashcash/go/exp/services/ledgerexporter/internal"

func main() {
	app := exporter.NewApp()
	app.Run()
}
