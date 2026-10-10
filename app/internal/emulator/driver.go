// Package emulator drives the Accio app on the MuMu emulator via adb for
// account creation and manual login flows.
package emulator

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Driver wraps adb calls against one device.
type Driver struct {
	ADBPath string
	Serial  string
	// Enabled desliga TODO spawn de adb (padrão: false). Quando false,
	// shell() não executa nada e IsAlive() devolve false.
	Enabled bool
}

func New(adbPath, serial string) *Driver {
	return &Driver{ADBPath: adbPath, Serial: serial}
}

// SetEnabled liga/desliga o driver em runtime (botão da UI).
func (d *Driver) SetEnabled(on bool) { d.Enabled = on }

func (d *Driver) shell(ctx context.Context, args string) (string, error) {
	if !d.Enabled {
		return "", fmt.Errorf("emulador desativado")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, d.ADBPath, "-s", d.Serial, "shell", args)
	hideCmd(cmd)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// IsAlive checks the device is reachable.
func (d *Driver) IsAlive(ctx context.Context) bool {
	out, _ := d.shell(ctx, "echo ok")
	return strings.TrimSpace(out) == "ok"
}

// AppPID returns the pid of the Accio app ("" when not running).
func (d *Driver) AppPID(ctx context.Context) string {
	out, _ := d.shell(ctx, "pidof com.accio.android.app")
	return strings.TrimSpace(strings.Fields(out)[0])
}

// LaunchApp force-stops and relaunches the Accio app.
func (d *Driver) LaunchApp(ctx context.Context) error {
	if _, err := d.shell(ctx, "am force-stop com.accio.android.app"); err != nil {
		return err
	}
	time.Sleep(1500 * time.Millisecond)
	out, err := d.shell(ctx, "monkey -p com.accio.android.app -c android.intent.category.LAUNCHER 1")
	if err != nil {
		return fmt.Errorf("launch: %v: %s", err, out)
	}
	time.Sleep(6 * time.Second)
	return nil
}

// Tap simulates a screen tap.
func (d *Driver) Tap(ctx context.Context, x, y int) error {
	_, err := d.shell(ctx, fmt.Sprintf("input tap %d %d", x, y))
	return err
}

// Text types text (ASCII-safe; adb input text limitation).
func (d *Driver) Text(ctx context.Context, s string) error {
	s = strings.ReplaceAll(s, " ", "%s")
	_, err := d.shell(ctx, "input text "+s)
	return err
}

// Key sends a keyevent (e.g. 66 = ENTER, 4 = BACK).
func (d *Driver) Key(ctx context.Context, code int) error {
	_, err := d.shell(ctx, fmt.Sprintf("input keyevent %d", code))
	return err
}

// DumpUI captures the uiautomator XML and returns text labels with bounds.
func (d *Driver) DumpUI(ctx context.Context) ([]UIElement, error) {
	if _, err := d.shell(ctx, "uiautomator dump /data/local/tmp/ui.xml"); err != nil {
		return nil, err
	}
	out, err := d.shell(ctx, "cat /data/local/tmp/ui.xml")
	if err != nil {
		return nil, err
	}
	return parseUI(out), nil
}

type UIElement struct {
	Text        string
	Desc        string
	Bounds      string
	X, Y        int
}

var nodeRe = regexp.MustCompile(`<node[^>]*text="([^"]*)"[^>]*content-desc="([^"]*)"[^>]*bounds="([^"]*)"[^>]*/?>`)

func parseUI(xml string) []UIElement {
	var out []UIElement
	for _, m := range nodeRe.FindAllStringSubmatch(xml, -1) {
		text, desc, bounds := m[1], m[2], m[3]
		if text == "" && desc == "" {
			continue
		}
		x, y := boundsCenter(bounds)
		out = append(out, UIElement{Text: text, Desc: desc, Bounds: bounds, X: x, Y: y})
	}
	return out
}

var boundsRe = regexp.MustCompile(`\[(\d+),(\d+)\]\[(\d+),(\d+)\]`)

func boundsCenter(bounds string) (int, int) {
	m := boundsRe.FindStringSubmatch(bounds)
	if m == nil {
		return 0, 0
	}
	var v [4]int
	for i := range v {
		fmt.Sscanf(m[i+1], "%d", &v[i])
	}
	return (v[0] + v[2]) / 2, (v[1] + v[3]) / 2
}

// TapLabel finds a UI element by exact text or content-desc and taps it.
func (d *Driver) TapLabel(ctx context.Context, label string) bool {
	elems, err := d.DumpUI(ctx)
	if err != nil {
		return false
	}
	for _, e := range elems {
		if e.Text == label || e.Desc == label {
			_ = d.Tap(ctx, e.X, e.Y)
			return true
		}
	}
	return false
}

// WaitForLabel polls the UI until a label appears (or timeout).
func (d *Driver) WaitForLabel(ctx context.Context, label string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		elems, err := d.DumpUI(ctx)
		if err == nil {
			for _, e := range elems {
				if e.Text == label || e.Desc == label {
					return true
				}
			}
		}
		time.Sleep(1200 * time.Millisecond)
	}
	return false
}

// LoginEmailFlow drives the manual login: focus the email field, type the
// email, tap "Enviar código". Returns false when the flow stalls.
func (d *Driver) LoginEmailFlow(ctx context.Context, email string) error {
	if err := d.LaunchApp(ctx); err != nil {
		return err
	}
	// Wait for the login screen (email field has the placeholder label on
	// some builds; fall back to fixed coordinates on the MuMu 720p layout).
	if !d.WaitForLabel(ctx, "Enviar código", 25*time.Second) {
		// fallback: tap the field by coordinates and verify text entry
		_ = d.Tap(ctx, 360, 525)
	}
	_ = d.Key(ctx, 4) // back (close keyboard)
	time.Sleep(600 * time.Millisecond)
	_ = d.Tap(ctx, 360, 525)
	time.Sleep(400 * time.Millisecond)
	if err := d.Text(ctx, email); err != nil {
		return err
	}
	_ = d.Key(ctx, 111) // ESC closes keyboard
	time.Sleep(400 * time.Millisecond)
	if !d.TapLabel(ctx, "Enviar código") {
		return fmt.Errorf("send-code button not found")
	}
	return nil
}

// EnterCodeFlow types the verification code and confirms.
func (d *Driver) EnterCodeFlow(ctx context.Context, code string) error {
	// The code screen shows individual digit boxes; tap the first and type.
	_ = d.Tap(ctx, 360, 400)
	time.Sleep(400 * time.Millisecond)
	if err := d.Text(ctx, code); err != nil {
		return err
	}
	time.Sleep(400 * time.Millisecond)
	_ = d.Key(ctx, 111)
	// Some builds need an explicit confirm button.
	_ = d.TapLabel(ctx, "Confirmar")
	return nil
}
