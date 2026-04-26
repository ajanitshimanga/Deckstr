package main

import (
	"context"
	_ "embed"

	"fyne.io/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

// startTray launches the system-tray icon. Discord-style behavior: the X
// button hides the window into the hidden-icons bar; left-click on the
// tray icon brings the window back; right-click opens a menu with
// Open / Quit. systray.Run blocks for the lifetime of the tray and locks
// its own OS thread internally, so it must be called on its own goroutine
// alongside wails.Run.
func (a *App) startTray() {
	onReady := func() {
		if len(trayIcon) > 0 {
			systray.SetIcon(trayIcon)
		}
		systray.SetTitle("Deckstr")
		systray.SetTooltip("Deckstr")

		// Left-click on the tray icon restores the window. Without this
		// the default behavior would pop the right-click menu on a left
		// click too, which doesn't match the Discord/Slack feel.
		systray.SetOnTapped(func() { a.showWindow() })

		openItem := systray.AddMenuItem("Open Deckstr", "Show the Deckstr window")
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("Quit Deckstr", "Quit Deckstr")

		go func() {
			for {
				select {
				case <-openItem.ClickedCh:
					a.showWindow()
				case <-quitItem.ClickedCh:
					a.requestQuit()
					return
				}
			}
		}()
	}

	systray.Run(onReady, func() {})
}

// showWindow restores the main window from the tray. No-op until the
// Wails runtime context is wired up by startup().
func (a *App) showWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// requestQuit flips the real-quit flag and asks the runtime to close.
// The flag tells beforeClose() to allow the close instead of hiding.
func (a *App) requestQuit() {
	a.quitting.Store(true)
	if a.ctx == nil {
		return
	}
	runtime.Quit(a.ctx)
}

// beforeClose runs whenever something tries to close the window — the X
// button on the custom title bar, ALT+F4, or runtime.Quit. We return
// true (prevent) so the runtime stays alive and the window just hides.
// The tray's Quit menu sets quitting=true first so this returns false
// and the app actually exits.
func (a *App) beforeClose(_ context.Context) bool {
	if a.quitting.Load() {
		return false
	}
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
	return true
}
