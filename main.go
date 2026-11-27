package main

import (
	"log"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hehehai/stopthat/internal/app"
	"github.com/hehehai/stopthat/internal/cli"
	"github.com/hehehai/stopthat/internal/firewall"
	"github.com/hehehai/stopthat/internal/store"
)

func main() {
	st, err := store.NewDefault()
	if err != nil {
		log.Printf("stopthat: config store fallback (in-memory): %v", err)
	}

	eng := firewall.NewEngine()
	if cli.ShouldHandle(os.Args[1:]) {
		os.Exit(cli.Run(os.Args[1:], eng, st, os.Stdin, os.Stdout, os.Stderr))
	}

	var win *mygo.Window

	state := app.NewState(eng, st, app.Options{
		Async: true,
		PickFolder: func(title string) (string, error) {
			paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
				Parent:    win,
				Title:     title,
				Directory: true,
			})
			if err != nil || len(paths) == 0 {
				return "", err
			}
			return paths[0], nil
		},
		PickFile: func(title string) (string, error) {
			paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
				Parent: win,
				Title:  title,
			})
			if err != nil || len(paths) == 0 {
				return "", err
			}
			return paths[0], nil
		},
	})

	mygo.App.WhenReady(func() {
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "StopThat — 应用联网管理",
			Width:     980,
			Height:    660,
			MinWidth:  780,
			MinHeight: 520,
			Content:   ui.View(state.View),
		})
		state.SetInvalidate(func() {
			if win != nil {
				win.Invalidate()
			}
		})
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
