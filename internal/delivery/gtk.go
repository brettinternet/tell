package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/godbus/dbus/v5"
)

// The Linux click helper is tell itself, registered by `task linux:install` as
// a D-Bus-activatable application. GNOME activates it on click, after the
// posting tell has exited, and passes the click actions as the action target.
const (
	desktopAppID   = "com.brettinternet.tell"
	desktopAppPath = dbus.ObjectPath("/com/brettinternet/tell")
	gtkName        = "org.gtk.Notifications"
)

// linuxHelperInstalled reports whether the desktop entry and D-Bus service
// that make tell activatable exist under ~/.local/share.
func linuxHelperInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	share := filepath.Join(home, ".local", "share")
	for _, path := range []string{
		filepath.Join(share, "applications", desktopAppID+".desktop"),
		filepath.Join(share, "dbus-1", "services", desktopAppID+".service"),
	} {
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// sessionBus connects to the inherited session bus; like notify-send, it never
// discovers or autolaunches one.
func sessionBus(ctx context.Context) (*dbus.Conn, error) {
	address := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if address == "" || address == "autolaunch:" {
		return nil, errors.New("no session bus")
	}
	return dbus.Connect(address, dbus.WithContext(ctx))
}

// gtkNotifications returns a session bus connection when the click helper is
// installed and the desktop shell serves org.gtk.Notifications (GNOME), or nil
// when the desktop route should use notify-send.
func gtkNotifications(ctx context.Context, timeout time.Duration) *dbus.Conn {
	if !linuxHelperInstalled() {
		return nil
	}
	conn, err := sessionBus(ctx)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var owned bool
	if err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, gtkName).Store(&owned); err != nil || !owned {
		_ = conn.Close()
		return nil
	}
	return conn
}

// notifyGTK posts as tell's desktop entry. Only click actions, never message
// text, are stored in the action target.
func notifyGTK(ctx context.Context, timeout time.Duration, conn *dbus.Conn, message Message, onClick []string) *Result {
	const route = "desktop"
	cwd, _ := os.Getwd()
	priority := "normal"
	if message.Type == "failure" {
		priority = "urgent"
	}
	notification := map[string]dbus.Variant{
		"title":    dbus.MakeVariant(message.Title),
		"body":     dbus.MakeVariant(message.Body),
		"priority": dbus.MakeVariant(priority),
	}
	if actions := clickActions(captureOrigin(os.Getenv, cwd), onClick, message.URL, "linux"); len(actions) > 0 {
		target, err := json.Marshal(actions)
		if err != nil {
			return NewResult(route, "failed", "provider_failure", "could not encode desktop notification")
		}
		notification["default-action"] = dbus.MakeVariant("app.click")
		notification["default-action-target"] = dbus.MakeVariant(string(target))
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	id := strconv.FormatInt(time.Now().UnixNano(), 36)
	call := conn.Object(gtkName, "/org/gtk/Notifications").CallWithContext(ctx, gtkName+".AddNotification", dbus.FlagNoAutoStart, desktopAppID, id, notification)
	if call.Err != nil {
		if ctx.Err() != nil {
			return NewResult(route, "uncertain", "timeout", "provider interrupted after start; delivery may have occurred")
		}
		return NewResult(route, "failed", "provider_failure", "desktop notification provider reported failure")
	}
	return NewResult(route, "accepted", "", "")
}

// clickApplication implements org.freedesktop.Application for activation.
type clickApplication struct{ clicks chan []clickAction }

func (app clickApplication) Activate(map[string]dbus.Variant) *dbus.Error { return nil }

func (app clickApplication) Open([]string, map[string]dbus.Variant) *dbus.Error { return nil }

func (app clickApplication) ActivateAction(name string, parameters []dbus.Variant, platform map[string]dbus.Variant) *dbus.Error {
	var target string
	if name != "click" || len(parameters) != 1 || parameters[0].Store(&target) != nil {
		return nil
	}
	var actions []clickAction
	if json.Unmarshal([]byte(target), &actions) != nil {
		return nil
	}
	// The activation token lets the opened window take focus on Wayland.
	var token string
	if value, ok := platform["activation-token"]; ok && value.Store(&token) == nil && token != "" {
		for i := range actions {
			actions[i].Env = append(actions[i].Env, "XDG_ACTIVATION_TOKEN="+token, "DESKTOP_STARTUP_ID="+token)
		}
	}
	select {
	case app.clicks <- actions:
	default:
	}
	return nil
}

// ServeDesktopClicks is the D-Bus-activated click helper. It runs each clicked
// notification's actions with the same rules as Tell.app (literal argv, 10
// seconds each, output discarded), then exits once no click arrives for a few
// seconds.
func ServeDesktopClicks(ctx context.Context) error {
	conn, err := sessionBus(ctx)
	if err != nil {
		return errors.New("session bus is not available")
	}
	defer func() { _ = conn.Close() }()
	app := clickApplication{clicks: make(chan []clickAction, 8)}
	if err := conn.Export(app, desktopAppPath, "org.freedesktop.Application"); err != nil {
		return errors.New("could not export the click helper")
	}
	if reply, err := conn.RequestName(desktopAppID, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		return errors.New("another click helper is running")
	}
	idle := time.NewTimer(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-idle.C:
			return nil
		case actions := <-app.clicks:
			for _, action := range actions {
				if len(action.Argv) == 0 {
					continue
				}
				env := append(os.Environ(), action.Env...)
				_, _ = RunCommand(ctx, 10*time.Second, "desktop", action.Argv[0], action.Argv[1:], nil, env, action.Dir)
			}
			idle.Reset(3 * time.Second)
		}
	}
}
