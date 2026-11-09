package main

import (
	"embed"
	"kiko/core/attachment"
	"log"
	"os"
	"strconv"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--render-pdf" {
		page, err := strconv.Atoi(os.Args[2])
		if err != nil {
			os.Exit(1)
		}
		if err = attachment.RenderWorker(os.Stdin, os.Stdout, page); err != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--extract-pdf" {
		if err := attachment.Worker(os.Stdin, os.Stdout); err != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--web" {
		if err := serveWeb(assets); err != nil {
			log.Fatal(err)
		}
		return
	}
	app := &App{}
	err := wails.Run(&options.App{
		Title: "Kiko", Width: 1200, Height: 800,
		MinWidth: 640, MinHeight: 480,
		BackgroundColour:   options.NewRGBA(24, 24, 24, 255),
		AssetServer:        &assetserver.Options{Assets: assets},
		Bind:               []interface{}{app},
		OnStartup:          app.startup,
		OnShutdown:         app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "kiko-desktop", OnSecondInstanceLaunch: func(_ options.SecondInstanceData) { runtime.WindowShow(app.ctx); runtime.WindowUnminimise(app.ctx) }},
	})
	if err != nil {
		log.Fatal(err)
	}
}
