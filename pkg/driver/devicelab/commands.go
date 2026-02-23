package devicelab

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/devicelab-dev/maestro-runner/pkg/core"
	"github.com/devicelab-dev/maestro-runner/pkg/flow"
	"github.com/devicelab-dev/maestro-runner/pkg/logger"
)

// Tap commands

func (d *Driver) tapOn(step *flow.TapOnStep) *core.CommandResult {
	// Screen-relative point tap (no selector)
	if step.Point != "" && step.Selector.IsEmpty() {
		return d.tapOnPointWithCoords(step.Point)
	}

	// Keyboard key shortcut
	if step.Selector.Text != "" {
		if keyChar := iosKeyboardKey(step.Selector.Text); keyChar != "" {
			if err := d.adapter.TypeText(keyChar); err != nil {
				return errorResult(err, fmt.Sprintf("Failed to send key: %s", step.Selector.Text))
			}
			return successResult(fmt.Sprintf("Pressed keyboard key: %s", step.Selector.Text), nil)
		}
	}

	// Combined find+tap RPC for performance
	params := map[string]interface{}{}
	if step.Selector.Text != "" {
		params["text"] = step.Selector.Text
	} else if step.Selector.ID != "" {
		params["id"] = step.Selector.ID
	}

	// For relative selectors or complex selectors, fall back to separate find+tap
	if step.Selector.HasRelativeSelector() || step.Selector.HasNonZeroIndex() || step.Point != "" {
		info, err := d.findElement(step.Selector, step.Optional, step.TimeoutMs)
		if err != nil {
			if step.Optional {
				return successResult("Optional element not found, skipping tap", nil)
			}
			return errorResult(err, fmt.Sprintf("Element not found: %s", selectorDesc(step.Selector)))
		}

		// Relative point within element
		if step.Point != "" && info.Bounds.Width > 0 {
			px, py, parseErr := core.ParsePointCoords(step.Point, info.Bounds.Width, info.Bounds.Height)
			if parseErr != nil {
				return errorResult(parseErr, "Invalid point coordinates")
			}
			x := float64(info.Bounds.X + px)
			y := float64(info.Bounds.Y + py)
			if err := d.adapter.Tap(x, y); err != nil {
				return errorResult(err, "Tap at relative point failed")
			}
			return successResult(fmt.Sprintf("Tapped at relative point (%.0f, %.0f)", x, y), info)
		}

		// Tap at center
		x := float64(info.Bounds.X + info.Bounds.Width/2)
		y := float64(info.Bounds.Y + info.Bounds.Height/2)
		if err := d.adapter.Tap(x, y); err != nil {
			return errorResult(err, "Tap failed")
		}
		return successResult("Tapped element", info)
	}

	// Use combined findAndTap RPC for simple selectors (perf win)
	timeout := d.calculateTimeout(step.Optional, step.TimeoutMs)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			if step.Optional {
				return successResult("Optional element not found, skipping tap", nil)
			}
			if lastErr != nil {
				return errorResult(lastErr, fmt.Sprintf("Element not found: %s", selectorDesc(step.Selector)))
			}
			return errorResult(ctx.Err(), fmt.Sprintf("Element not found: %s", selectorDesc(step.Selector)))
		default:
			result, err := d.adapter.FindAndTap(params)
			if err == nil {
				return successResult("Tapped element", d.elementResultToInfo(result))
			}
			lastErr = err
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func (d *Driver) tapOnPointWithCoords(point string) *core.CommandResult {
	width, height, err := d.screenSize()
	if err != nil {
		return errorResult(err, "Failed to get screen size")
	}

	x, y, err := core.ParsePointCoords(point, width, height)
	if err != nil {
		return errorResult(err, fmt.Sprintf("Invalid point coordinates: %s", point))
	}

	if err := d.adapter.Tap(float64(x), float64(y)); err != nil {
		return errorResult(err, "Tap at point failed")
	}

	return successResult(fmt.Sprintf("Tapped at (%d, %d)", x, y), nil)
}

func (d *Driver) doubleTapOn(step *flow.DoubleTapOnStep) *core.CommandResult {
	info, err := d.findElement(step.Selector, false, step.TimeoutMs)
	if err != nil {
		return errorResult(err, fmt.Sprintf("Element not found: %s", selectorDesc(step.Selector)))
	}

	x := float64(info.Bounds.X + info.Bounds.Width/2)
	y := float64(info.Bounds.Y + info.Bounds.Height/2)

	if err := d.adapter.DoubleTap(x, y); err != nil {
		return errorResult(err, "Double tap failed")
	}

	return successResult("Double tapped element", info)
}

func (d *Driver) longPressOn(step *flow.LongPressOnStep) *core.CommandResult {
	info, err := d.findElement(step.Selector, false, step.TimeoutMs)
	if err != nil {
		return errorResult(err, fmt.Sprintf("Element not found: %s", selectorDesc(step.Selector)))
	}

	x := float64(info.Bounds.X + info.Bounds.Width/2)
	y := float64(info.Bounds.Y + info.Bounds.Height/2)

	if err := d.adapter.LongPress(x, y, 1.0); err != nil {
		return errorResult(err, "Long press failed")
	}

	return successResult("Long pressed element", info)
}

func (d *Driver) tapOnPoint(step *flow.TapOnPointStep) *core.CommandResult {
	var x, y float64

	if step.Point != "" {
		width, height, err := d.screenSize()
		if err != nil {
			return errorResult(err, "Failed to get screen size")
		}
		px, py, err := core.ParsePointCoords(step.Point, width, height)
		if err != nil {
			return errorResult(err, "Invalid point format")
		}
		x = float64(px)
		y = float64(py)
	} else {
		x = float64(step.X)
		y = float64(step.Y)
	}

	if err := d.adapter.Tap(x, y); err != nil {
		return errorResult(err, "Tap on point failed")
	}

	return successResult(fmt.Sprintf("Tapped at (%.0f, %.0f)", x, y), nil)
}

// Assert commands

func (d *Driver) assertVisible(step *flow.AssertVisibleStep) *core.CommandResult {
	info, err := d.findElement(step.Selector, step.IsOptional(), step.TimeoutMs)
	if err != nil {
		return errorResult(err, fmt.Sprintf("Element not visible: %s", selectorDesc(step.Selector)))
	}

	return successResult("Element is visible", info)
}

func (d *Driver) assertNotVisible(step *flow.AssertNotVisibleStep) *core.CommandResult {
	timeoutMs := step.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}

	info, err := d.findElement(step.Selector, true, timeoutMs)
	if err != nil || info == nil {
		return successResult("Element is not visible", nil)
	}

	return errorResult(fmt.Errorf("element is visible"), fmt.Sprintf("Element should not be visible: %s", selectorDesc(step.Selector)))
}

// Input commands

func (d *Driver) inputText(step *flow.InputTextStep) *core.CommandResult {
	text := step.Text
	if text == "" {
		return errorResult(fmt.Errorf("no text specified"), "No text to input")
	}

	unicodeWarning := ""
	if core.HasNonASCII(text) {
		unicodeWarning = " (warning: non-ASCII characters may not input correctly)"
	}

	// If selector provided, find and tap to focus first
	if !step.Selector.IsEmpty() {
		info, err := d.findElement(step.Selector, step.IsOptional(), step.TimeoutMs)
		if err != nil {
			return errorResult(err, fmt.Sprintf("Element not found: %s", selectorDesc(step.Selector)))
		}
		x := float64(info.Bounds.X + info.Bounds.Width/2)
		y := float64(info.Bounds.Y + info.Bounds.Height/2)
		if err := d.adapter.Tap(x, y); err != nil {
			return errorResult(err, "Failed to tap element before input")
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for text field focus before typing. After a tapOn, the text field
	// needs time to become first responder (especially the first tap after app
	// launch which initializes the keyboard system). Poll for focused element
	// similar to WDA's GetActiveElement polling.
	for i := 0; i < 5; i++ {
		if focused, err := d.adapter.HasFocusedElement(); err == nil && focused {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if err := d.adapter.TypeText(text); err != nil {
		return errorResult(err, "Input text failed")
	}

	return successResult(fmt.Sprintf("Entered text: %s%s", text, unicodeWarning), nil)
}

func (d *Driver) eraseText(step *flow.EraseTextStep) *core.CommandResult {
	chars := step.Characters
	if chars == 0 {
		chars = 50
	}

	if err := d.adapter.EraseText(chars); err != nil {
		return errorResult(err, "Erase text failed")
	}

	return successResult(fmt.Sprintf("Erased %d characters", chars), nil)
}

func (d *Driver) hideKeyboard(step *flow.HideKeyboardStep) *core.CommandResult {
	_ = d.adapter.HideKeyboard()
	return successResult("Attempted to hide keyboard", nil)
}

func (d *Driver) acceptAlert(step *flow.AcceptAlertStep) *core.CommandResult {
	return d.waitForAlert(step.TimeoutMs, true)
}

func (d *Driver) dismissAlert(step *flow.DismissAlertStep) *core.CommandResult {
	return d.waitForAlert(step.TimeoutMs, false)
}

func (d *Driver) waitForAlert(timeoutMs int, accept bool) *core.CommandResult {
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	action := "accept"
	if !accept {
		action = "dismiss"
	}

	for {
		select {
		case <-ctx.Done():
			return successResult(fmt.Sprintf("No alert to %s", action), nil)
		default:
			var err error
			if accept {
				err = d.adapter.AcceptAlert()
			} else {
				err = d.adapter.DismissAlert()
			}
			if err == nil {
				return successResult(fmt.Sprintf("Alert %sed", action), nil)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (d *Driver) inputRandom(step *flow.InputRandomStep) *core.CommandResult {
	length := step.Length
	if length <= 0 {
		length = 10
	}

	var text string
	dataType := strings.ToUpper(step.DataType)
	switch dataType {
	case "EMAIL":
		text = core.RandomEmail()
	case "NUMBER":
		text = core.RandomNumber(length)
	case "PERSON_NAME":
		text = core.RandomPersonName()
	default:
		text = core.RandomString(length)
	}

	if err := d.adapter.TypeText(text); err != nil {
		return errorResult(err, "Input random text failed")
	}

	return &core.CommandResult{
		Success: true,
		Message: fmt.Sprintf("Entered random %s: %s", dataType, text),
		Data:    text,
	}
}

// Scroll/Swipe commands

func (d *Driver) scroll(step *flow.ScrollStep) *core.CommandResult {
	if err := d.adapter.Scroll(step.Direction); err != nil {
		return errorResult(err, "Scroll failed")
	}
	return successResult(fmt.Sprintf("Scrolled %s", step.Direction), nil)
}

func (d *Driver) scrollUntilVisible(step *flow.ScrollUntilVisibleStep) *core.CommandResult {
	timeout := d.calculateTimeout(false, step.TimeoutMs)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	maxScrolls := step.MaxScrolls
	if maxScrolls <= 0 {
		maxScrolls = 10
	}

	direction := step.Direction
	if direction == "" {
		direction = "down"
	}

	for i := 0; i < maxScrolls; i++ {
		// Check if element is visible
		info, err := d.findElementOnce(step.Element)
		if err == nil {
			return successResult("Element found after scrolling", info)
		}

		select {
		case <-ctx.Done():
			return errorResult(ctx.Err(), fmt.Sprintf("Element not found after %d scrolls", i))
		default:
		}

		// Scroll
		if err := d.adapter.Scroll(direction); err != nil {
			return errorResult(err, "Scroll failed")
		}
		time.Sleep(300 * time.Millisecond)
	}

	return errorResult(fmt.Errorf("element not found after %d scrolls", maxScrolls),
		fmt.Sprintf("Element not found: %s", selectorDesc(step.Element)))
}

func (d *Driver) swipe(step *flow.SwipeStep) *core.CommandResult {
	width, height, err := d.screenSize()
	if err != nil {
		return errorResult(err, "Failed to get screen size")
	}

	centerX := float64(width) / 2
	centerY := float64(height) / 2
	swipeDist := float64(height) / 3

	var fromX, fromY, toX, toY float64

	if step.Start != "" && step.End != "" {
		sx, sy, err := core.ParsePointCoords(step.Start, width, height)
		if err != nil {
			return errorResult(err, "Invalid start coordinates")
		}
		ex, ey, err := core.ParsePointCoords(step.End, width, height)
		if err != nil {
			return errorResult(err, "Invalid end coordinates")
		}
		fromX, fromY = float64(sx), float64(sy)
		toX, toY = float64(ex), float64(ey)
	} else {
		switch strings.ToLower(step.Direction) {
		case "up":
			fromX, fromY = centerX, centerY+swipeDist/2
			toX, toY = centerX, centerY-swipeDist/2
		case "down":
			fromX, fromY = centerX, centerY-swipeDist/2
			toX, toY = centerX, centerY+swipeDist/2
		case "left":
			fromX, fromY = centerX+swipeDist/2, centerY
			toX, toY = centerX-swipeDist/2, centerY
		case "right":
			fromX, fromY = centerX-swipeDist/2, centerY
			toX, toY = centerX+swipeDist/2, centerY
		default:
			return errorResult(fmt.Errorf("unknown swipe direction: %s", step.Direction), "Unknown direction")
		}
	}

	if err := d.adapter.Swipe(fromX, fromY, toX, toY); err != nil {
		return errorResult(err, "Swipe failed")
	}

	return successResult(fmt.Sprintf("Swiped %s", step.Direction), nil)
}

// Navigation commands

func (d *Driver) back(step *flow.BackStep) *core.CommandResult {
	return errorResult(fmt.Errorf("back not supported on iOS"), "iOS doesn't have a back button")
}

func (d *Driver) pressKey(step *flow.PressKeyStep) *core.CommandResult {
	switch strings.ToLower(step.Key) {
	case "home":
		if err := d.adapter.PressHome(); err != nil {
			return errorResult(err, "Press home failed")
		}
	case "volumeup", "volume_up":
		if err := d.adapter.PressButton("volumeUp"); err != nil {
			return errorResult(err, "Press volume up failed")
		}
	case "volumedown", "volume_down":
		if err := d.adapter.PressButton("volumeDown"); err != nil {
			return errorResult(err, "Press volume down failed")
		}
	default:
		if keyChar := iosKeyboardKey(step.Key); keyChar != "" {
			if err := d.adapter.TypeText(keyChar); err != nil {
				return errorResult(err, fmt.Sprintf("Press %s failed", step.Key))
			}
		} else {
			return errorResult(fmt.Errorf("unknown key: %s", step.Key), "Unknown key")
		}
	}

	return successResult(fmt.Sprintf("Pressed %s", step.Key), nil)
}

func iosKeyboardKey(name string) string {
	switch strings.ToLower(name) {
	case "return", "enter":
		return "\n"
	case "tab":
		return "\t"
	case "delete", "backspace":
		return "\b"
	case "space":
		return " "
	default:
		return ""
	}
}

// App lifecycle

func (d *Driver) launchApp(step *flow.LaunchAppStep) *core.CommandResult {
	bundleID := step.AppID
	if bundleID == "" {
		return errorResult(fmt.Errorf("bundleID required"), "Bundle ID is required for launchApp")
	}

	// Handle permissions
	permissions := step.Permissions
	if d.udid != "" && len(permissions) == 0 {
		permissions = map[string]string{"all": "allow"}
	}
	needPerms := d.udid != "" && d.info.IsSimulator && !hasAllValue(permissions, "unset")

	if step.ClearState {
		_ = d.adapter.TerminateApp(bundleID)

		if needPerms {
			allPerms := getIOSPermissions()
			var wg sync.WaitGroup
			var clearResult *core.CommandResult

			wg.Add(1)
			go func() {
				defer wg.Done()
				clearResult = d.clearAppState(bundleID)
			}()

			wg.Add(len(allPerms))
			for _, perm := range allPerms {
				go func(p string) {
					defer wg.Done()
					_ = d.resetIOSPermission(bundleID, p)
				}(perm)
			}
			wg.Wait()

			if clearResult != nil && !clearResult.Success {
				return clearResult
			}

			// Grant permissions
			d.applyPermissions(bundleID, permissions)
		} else {
			if result := d.clearAppState(bundleID); !result.Success {
				return result
			}
		}
	} else if needPerms {
		allPerms := getIOSPermissions()
		var wg sync.WaitGroup
		wg.Add(len(allPerms))
		for _, perm := range allPerms {
			go func(p string) {
				defer wg.Done()
				_ = d.resetIOSPermission(bundleID, p)
			}(perm)
		}
		wg.Wait()

		d.applyPermissions(bundleID, permissions)
	}

	if d.udid != "" {
		d.alertAction = resolveAlertAction(permissions)
	}

	// Convert arguments
	var launchArgs []string
	launchEnv := make(map[string]string)

	for key, value := range step.Environment {
		launchEnv[key] = value
	}

	if len(step.Arguments) > 0 {
		for key, value := range step.Arguments {
			var strVal string
			switch v := value.(type) {
			case string:
				strVal = v
			case bool:
				if v {
					strVal = "true"
				} else {
					strVal = "false"
				}
			default:
				strVal = fmt.Sprint(v)
			}
			// Don't double-prefix: YAML keys like "--username" already have dashes
			argKey := key
			if !strings.HasPrefix(key, "-") {
				argKey = "-" + key
			}
			launchArgs = append(launchArgs, argKey, strVal)
			launchEnv[key] = strVal
		}
	}

	// Create session if not active
	if !d.sessionCreated {
		_, err := d.adapter.CreateSession(bundleID, d.alertAction, launchArgs, launchEnv)
		if err != nil {
			return errorResult(err, fmt.Sprintf("Failed to create session for app: %s", bundleID))
		}
		d.sessionCreated = true

		if len(launchArgs) == 0 && len(launchEnv) == 0 {
			return successResult(fmt.Sprintf("Launched app: %s", bundleID), nil)
		}
	}

	// Launch with args
	if err := d.adapter.LaunchApp(bundleID, launchArgs, launchEnv); err != nil {
		return errorResult(err, fmt.Sprintf("Failed to launch app: %s", bundleID))
	}

	return successResult(fmt.Sprintf("Launched app: %s", bundleID), nil)
}

func (d *Driver) applyPermissions(bundleID string, permissions map[string]string) {
	allPerms := getIOSPermissions()
	var applyList []struct{ perm, action string }
	for name, value := range permissions {
		lower := strings.ToLower(value)
		if lower != "allow" && lower != "deny" {
			continue
		}
		if strings.ToLower(name) == "all" {
			for _, perm := range allPerms {
				applyList = append(applyList, struct{ perm, action string }{perm, lower})
			}
		} else {
			for _, perm := range resolveIOSPermissionShortcut(name) {
				applyList = append(applyList, struct{ perm, action string }{perm, lower})
			}
		}
	}
	var wg sync.WaitGroup
	wg.Add(len(applyList))
	for _, item := range applyList {
		go func(p, a string) {
			defer wg.Done()
			_ = d.applyIOSPermission(bundleID, p, a)
		}(item.perm, item.action)
	}
	wg.Wait()
}

func (d *Driver) stopApp(step *flow.StopAppStep) *core.CommandResult {
	bundleID := step.AppID
	if bundleID == "" {
		return errorResult(fmt.Errorf("bundleID required"), "Bundle ID is required for stopApp")
	}
	if err := d.adapter.TerminateApp(bundleID); err != nil {
		return errorResult(err, fmt.Sprintf("Failed to stop app: %s", bundleID))
	}
	return successResult(fmt.Sprintf("Stopped app: %s", bundleID), nil)
}

func (d *Driver) killApp(step *flow.KillAppStep) *core.CommandResult {
	bundleID := step.AppID
	if bundleID == "" {
		return errorResult(fmt.Errorf("bundleID required"), "Bundle ID is required for killApp")
	}
	if err := d.adapter.TerminateApp(bundleID); err != nil {
		return errorResult(err, fmt.Sprintf("Failed to kill app: %s", bundleID))
	}
	return successResult(fmt.Sprintf("Killed app: %s", bundleID), nil)
}

func (d *Driver) clearState(step *flow.ClearStateStep) *core.CommandResult {
	bundleID := step.AppID
	if bundleID == "" {
		return errorResult(fmt.Errorf("bundleID required"), "Bundle ID is required for clearState")
	}
	_ = d.adapter.TerminateApp(bundleID)
	return d.clearAppState(bundleID)
}

func (d *Driver) clearAppState(bundleID string) *core.CommandResult {
	if d.appFile == "" {
		return errorResult(fmt.Errorf("clearState on iOS requires --app-file for reinstall"),
			"clearState on iOS requires --app-file to reinstall the app after uninstalling\n"+
				"Usage: maestro-runner --app-file <path-to-ipa-or-app> --platform ios test <flow-files>")
	}

	if d.info.IsSimulator {
		return d.clearAppStateSimulator(bundleID)
	}
	return d.clearAppStateDevice(bundleID)
}

func (d *Driver) clearAppStateSimulator(bundleID string) *core.CommandResult {
	cmd := exec.Command("xcrun", "simctl", "uninstall", d.udid, bundleID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return errorResult(fmt.Errorf("simctl uninstall failed: %w: %s", err, string(output)),
			"Failed to uninstall app on simulator")
	}

	cmd = exec.Command("xcrun", "simctl", "install", d.udid, d.appFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return errorResult(fmt.Errorf("simctl install failed: %w: %s", err, string(output)),
			"Failed to reinstall app on simulator")
	}

	return successResult(fmt.Sprintf("Cleared state for %s (uninstall+reinstall)", bundleID), nil)
}

func (d *Driver) clearAppStateDevice(bundleID string) *core.CommandResult {
	// Use xcrun devicectl to uninstall on physical device
	cmd := exec.Command("xcrun", "devicectl", "device", "uninstall", "app",
		"--device", d.udid, bundleID)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("devicectl uninstall: %s", string(output))
	}

	// Reinstall using go-ios
	cmd = exec.Command("xcrun", "devicectl", "device", "install", "app",
		"--device", d.udid, d.appFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return errorResult(fmt.Errorf("devicectl install failed: %w: %s", err, string(output)),
			"Failed to reinstall app on device")
	}

	return successResult(fmt.Sprintf("Cleared state for %s (uninstall+reinstall)", bundleID), nil)
}

// Clipboard commands

func (d *Driver) copyTextFrom(step *flow.CopyTextFromStep) *core.CommandResult {
	info, err := d.findElement(step.Selector, false, step.TimeoutMs)
	if err != nil {
		return errorResult(err, fmt.Sprintf("Element not found: %s", selectorDesc(step.Selector)))
	}

	return &core.CommandResult{
		Success: true,
		Message: fmt.Sprintf("Copied text: %s", info.Text),
		Data:    info.Text,
		Element: info,
	}
}

func (d *Driver) pasteText(step *flow.PasteTextStep) *core.CommandResult {
	text, err := d.adapter.GetClipboard()
	if err != nil {
		return errorResult(err, "Failed to get clipboard")
	}

	if err := d.adapter.TypeText(text); err != nil {
		return errorResult(err, "Failed to paste text")
	}

	return successResult(fmt.Sprintf("Pasted text: %s", text), nil)
}

func (d *Driver) setClipboard(step *flow.SetClipboardStep) *core.CommandResult {
	if err := d.adapter.SetClipboard(step.Text); err != nil {
		return errorResult(err, "Failed to set clipboard")
	}
	return successResult(fmt.Sprintf("Set clipboard: %s", step.Text), nil)
}

// Device control

func (d *Driver) setOrientation(step *flow.SetOrientationStep) *core.CommandResult {
	if err := d.adapter.SetOrientation(step.Orientation); err != nil {
		return errorResult(err, "Failed to set orientation")
	}
	return successResult(fmt.Sprintf("Orientation set to %s", step.Orientation), nil)
}

func (d *Driver) openLink(step *flow.OpenLinkStep) *core.CommandResult {
	if err := d.adapter.OpenURL(step.Link); err != nil {
		return errorResult(err, "Failed to open link")
	}
	return successResult(fmt.Sprintf("Opened link: %s", step.Link), nil)
}

func (d *Driver) openBrowser(step *flow.OpenBrowserStep) *core.CommandResult {
	if err := d.adapter.OpenURL(step.URL); err != nil {
		return errorResult(err, "Failed to open browser")
	}
	return successResult(fmt.Sprintf("Opened browser: %s", step.URL), nil)
}

// Wait commands

func (d *Driver) waitUntil(step *flow.WaitUntilStep) *core.CommandResult {
	timeoutMs := step.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = DefaultFindTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	if step.Visible != nil {
		// Wait until element becomes visible
		for {
			select {
			case <-ctx.Done():
				return errorResult(ctx.Err(), fmt.Sprintf("Wait condition not met: element not visible: %s", step.Visible.Describe()))
			default:
				info, err := d.findElementOnce(*step.Visible)
				if err == nil {
					return successResult("Wait condition met: element visible", info)
				}
				time.Sleep(200 * time.Millisecond)
			}
		}
	}

	if step.NotVisible != nil {
		// Wait until element is not visible
		for {
			select {
			case <-ctx.Done():
				return errorResult(ctx.Err(), fmt.Sprintf("Wait condition not met: element still visible: %s", step.NotVisible.Describe()))
			default:
				_, err := d.findElementOnce(*step.NotVisible)
				if err != nil {
					return successResult("Wait condition met: element not visible", nil)
				}
				time.Sleep(200 * time.Millisecond)
			}
		}
	}

	return errorResult(fmt.Errorf("no wait condition specified"), "waitUntil requires visible or notVisible selector")
}

func (d *Driver) waitForAnimationToEnd(step *flow.WaitForAnimationToEndStep) *core.CommandResult {
	// XCTest's quiescence detection runs implicitly on every snapshot/query,
	// so every subsequent command (findElement, getSource, etc.) already waits
	// for animations to settle. No explicit sleep or snapshot needed here.
	return successResult("Animation ended", nil)
}

// Media

func (d *Driver) takeScreenshot(step *flow.TakeScreenshotStep) *core.CommandResult {
	data, err := d.adapter.Screenshot()
	if err != nil {
		return errorResult(err, "Failed to take screenshot")
	}

	return &core.CommandResult{
		Success: true,
		Message: "Screenshot captured",
		Data:    data,
	}
}

// Permissions

func (d *Driver) setPermissions(step *flow.SetPermissionsStep) *core.CommandResult {
	appID := step.AppID
	if appID == "" {
		return errorResult(fmt.Errorf("no appId specified"), "No app ID for permissions")
	}

	if d.udid == "" {
		return successResult("setPermissions skipped (no UDID)", nil)
	}

	if len(step.Permissions) == 0 {
		return errorResult(fmt.Errorf("no permissions specified"), "No permissions to set")
	}

	if !d.info.IsSimulator {
		action := resolveAlertAction(step.Permissions)
		if action != "" {
			_ = d.adapter.SetAlertAction(action)
		}
		return successResult("setPermissions on real device: handled by alert monitor", nil)
	}

	if hasAllValue(step.Permissions, "unset") {
		return successResult("setPermissions: unset — no permissions changed", nil)
	}

	for _, perm := range getIOSPermissions() {
		_ = d.resetIOSPermission(appID, perm)
	}

	var applied, errors []string
	for name, value := range step.Permissions {
		lower := strings.ToLower(value)
		if lower != "allow" && lower != "deny" {
			continue
		}
		if strings.ToLower(name) == "all" {
			for _, perm := range getIOSPermissions() {
				if err := d.applyIOSPermission(appID, perm, lower); err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", perm, err))
				} else {
					applied = append(applied, perm)
				}
			}
		} else {
			for _, perm := range resolveIOSPermissionShortcut(name) {
				if err := d.applyIOSPermission(appID, perm, lower); err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", perm, err))
				} else {
					applied = append(applied, perm)
				}
			}
		}
	}

	msg := fmt.Sprintf("Permissions set: %d applied, all others reset", len(applied))
	if len(errors) > 0 {
		msg += fmt.Sprintf(", %d errors", len(errors))
	}

	return successResult(msg, nil)
}

// Permission helpers

func (d *Driver) applyIOSPermission(appID, permission, value string) error {
	var action string
	switch strings.ToLower(value) {
	case "allow":
		action = "grant"
	case "deny":
		action = "revoke"
	case "unset":
		action = "reset"
	default:
		return fmt.Errorf("invalid permission value: %s", value)
	}

	cmd := exec.Command("xcrun", "simctl", "privacy", d.udid, action, permission, appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

func (d *Driver) resetIOSPermission(appID, permission string) error {
	cmd := exec.Command("xcrun", "simctl", "privacy", d.udid, "reset", permission, appID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

func resolveIOSPermissionShortcut(shortcut string) []string {
	switch strings.ToLower(shortcut) {
	case "location", "location-always":
		return []string{"location-always"}
	case "camera":
		return []string{"camera"}
	case "contacts":
		return []string{"contacts"}
	case "phone":
		return []string{"contacts"}
	case "microphone":
		return []string{"microphone"}
	case "photos", "medialibrary":
		return []string{"photos"}
	case "calendar":
		return []string{"calendar"}
	case "reminders":
		return []string{"reminders"}
	case "notifications":
		return []string{"notifications"}
	case "bluetooth":
		return []string{"bluetooth-peripheral"}
	case "health":
		return []string{"health"}
	case "homekit":
		return []string{"homekit"}
	case "motion":
		return []string{"motion"}
	case "speech":
		return []string{"speech-recognition"}
	case "siri":
		return []string{"siri"}
	case "faceid":
		return []string{"faceid"}
	default:
		return []string{shortcut}
	}
}

func hasAllValue(permissions map[string]string, value string) bool {
	for _, v := range permissions {
		if strings.ToLower(v) != value {
			return false
		}
	}
	return len(permissions) > 0
}

func resolveAlertAction(permissions map[string]string) string {
	if len(permissions) == 0 {
		return "accept"
	}
	if val, ok := permissions["all"]; ok && len(permissions) == 1 {
		switch strings.ToLower(val) {
		case "allow":
			return "accept"
		case "deny":
			return "dismiss"
		}
	}
	var lastVal string
	for _, v := range permissions {
		lower := strings.ToLower(v)
		if lastVal == "" {
			lastVal = lower
		} else if lastVal != lower {
			return ""
		}
	}
	switch lastVal {
	case "allow":
		return "accept"
	case "deny":
		return "dismiss"
	default:
		return ""
	}
}

func getIOSPermissions() []string {
	return []string{
		"location-always",
		"camera",
		"microphone",
		"photos",
		"contacts",
		"calendar",
		"reminders",
		"notifications",
	}
}
