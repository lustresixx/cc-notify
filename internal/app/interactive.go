package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"cc-notify/internal/event"
	"cc-notify/internal/notifier"
)

const (
	colorReset   = "\x1b[0m"
	colorCyan    = "\x1b[96m" // bright cyan
	colorGreen   = "\x1b[92m" // bright green
	colorRed     = "\x1b[91m" // bright red
	colorYellow  = "\x1b[93m" // bright yellow
	colorDim     = "\x1b[2m"
	colorBold    = "\x1b[1m"
	colorMagenta = "\x1b[95m" // bright magenta
	colorWhite   = "\x1b[97m" // bright white
	colorBgDim   = "\x1b[100m"

	// Unicode symbols
	symBullet   = "●"
	symCircle   = "○"
	symArrow    = "❯"
	symDot      = "·"
	symBar      = "│"
	symCornerTL = "╭"
	symCornerTR = "╮"
	symHLine    = "─"
	symRadioOn  = "◉"
	symRadioOff = "○"
	symCheckBox = "☑"
	symUncheck  = "☐"
	symSpark    = "⚡"
	symGear     = "⚙"
	symBell     = "🔔"
	symDisk     = "💾"
	symPlug     = "🔌"
	symWave     = "👋"
)

// version is the tool version shown in the header.
const version = "v0.4.1"

const (
	tabDefault = 0
	tabCodex   = 1
	tabClaude  = 2
)

type keyCode int

const (
	keyUnknown keyCode = iota
	keyUp
	keyDown
	keyLeft
	keyRight
	keyEnter
	keySpace
	keyEsc
)

func (a *App) runInteractive() error {
	prefs, exists, err := a.loadPreferences()
	if err != nil {
		return err
	}

	if !exists || !prefs.SetupDone {
		a.renderSetupBanner()
		fmt.Fprintln(a.stdout, "  First launch detected. Auto-configuring hooks...")
		fmt.Fprintln(a.stdout)

		if err := a.runInstall(nil); err != nil {
			fmt.Fprintf(a.stderr, "  %s%s note:%s auto install failed: %v\n", colorBold, colorYellow, colorReset, err)
		}

		prefs.SetupDone = true
		if saveErr := a.savePreferences(prefs); saveErr != nil {
			fmt.Fprintf(a.stderr, "  %s%s note:%s save setup state failed: %v\n", colorBold, colorYellow, colorReset, saveErr)
		}
	}

	if a.stdinIsTTY() && a.stdoutIsTTY() {
		if err := a.runInteractiveKeyUI(&prefs); err == nil {
			return nil
		}
	}
	return a.runInteractiveLineUI(&prefs)
}

func (a *App) renderSetupBanner() {
	fmt.Fprintln(a.stdout)
	fmt.Fprintln(a.stdout)
	fmt.Fprintf(a.stdout, "  %s%s ⚡ CODEX NOTIFIED %s  %s%sInitial Setup%s\n",
		colorBold, colorCyan, colorReset,
		colorBgDim, colorWhite, colorReset)
	fmt.Fprintf(a.stdout, "  %s%s%s\n", colorDim, strings.Repeat(symHLine, 50), colorReset)
	fmt.Fprintln(a.stdout)
}

func (a *App) runInteractiveKeyUI(prefs *Preferences) error {
	restore, ok := enableRawInput(a.stdin, a.stdout)
	if !ok {
		return fmt.Errorf("raw input unavailable")
	}
	defer restore()

	tabNames := []string{"Default", "Codex", "Claude Code"}

	tab := tabDefault
	cursor := 0
	status := ""
	reader := bufio.NewReader(a.stdin)

	for {
		items := a.tabMenuItems(tab, *prefs)
		if cursor >= len(items) {
			cursor = 0
		}

		clearScreen(a.stdout)
		a.renderHeader()

		// ── Tab bar ──
		fmt.Fprintf(a.stdout, "  ")
		for i, name := range tabNames {
			if i == tab {
				fmt.Fprintf(a.stdout, " %s%s %s %s", colorBold+colorCyan, symCornerTL+symHLine, name, symHLine+symCornerTR+colorReset)
			} else {
				fmt.Fprintf(a.stdout, " %s %s %s", colorDim, name, colorReset)
			}
		}
		fmt.Fprintln(a.stdout)
		fmt.Fprintf(a.stdout, "  %s%s%s\n", colorDim, strings.Repeat(symHLine, 52), colorReset)

		// ── Tab description ──
		a.renderTabInfo(tab, *prefs)

		// ── Status message ──
		if status != "" {
			fmt.Fprintf(a.stdout, "  %s %s\n", symArrow, status)
		}

		// ── Menu items ──
		fmt.Fprintln(a.stdout)
		for i, item := range items {
			if i == cursor {
				fmt.Fprintf(a.stdout, "  %s%s%s %s%s%s\n", colorCyan, symArrow, colorReset, colorBold, item.label, colorReset)
			} else {
				fmt.Fprintf(a.stdout, "    %s%s%s\n", colorDim, item.label, colorReset)
			}
		}

		// ── Bottom bar ──
		fmt.Fprintln(a.stdout)
		fmt.Fprintf(a.stdout, "  %s%s%s\n", colorDim, strings.Repeat(symHLine, 52), colorReset)
		fmt.Fprintf(a.stdout, "  %s←/→%s tab  %s↑/↓%s navigate  %s⏎%s select  %sesc%s quit\n",
			colorBold, colorReset,
			colorBold, colorReset,
			colorBold, colorReset,
			colorBold, colorReset)

		key, err := readKey(reader)
		if err != nil {
			return err
		}
		switch key {
		case keyLeft:
			if tab > 0 {
				tab--
				cursor = 0
				status = ""
			}
		case keyRight:
			if tab < len(tabNames)-1 {
				tab++
				cursor = 0
				status = ""
			}
		case keyUp:
			if cursor > 0 {
				cursor--
			}
		case keyDown:
			if cursor < len(items)-1 {
				cursor++
			}
		case keyEnter:
			action := items[cursor].action
			if action == nil {
				continue
			}
			result := action(prefs)
			if result == actionExit {
				return nil
			}
			status = result.status
		case keyEsc:
			if err := a.savePreferences(*prefs); err != nil {
				fmt.Fprintf(a.stderr, "  %snote:%s save on exit failed: %v\n", colorYellow, colorReset, err)
			}
			clearScreen(a.stdout)
			fmt.Fprintf(a.stdout, "  %s%sGoodbye!%s\n\n", colorDim, colorCyan, colorReset)
			return nil
		}
	}
}

// actionResult holds the result of a menu action.
type actionResult struct {
	status string
	exit   bool
}

var actionExit = actionResult{exit: true}

type menuItem struct {
	label  string
	action func(prefs *Preferences) actionResult
}

func (a *App) renderTabInfo(tab int, p Preferences) {
	switch tab {
	case tabDefault:
		statusPill := fmt.Sprintf("%s%s%s ON%s", colorBold, colorGreen, symBullet, colorReset)
		if !p.Enabled {
			statusPill = fmt.Sprintf("%s%s OFF%s", colorDim, symCircle, colorReset)
		}
		fmt.Fprintf(a.stdout, "  %s%s%sGlobal defaults applied to all tools%s\n",
			colorDim, symBar, " ", colorReset)
		fmt.Fprintf(a.stdout, "  %s%s%s %s  mode:%s%s%s  content:%s%s%s\n",
			colorDim, symBar, colorReset,
			statusPill,
			colorBold, p.Mode, colorReset,
			colorBold, p.Content, colorReset)
	case tabCodex:
		en, mode, content := p.ToolPrefs("codex")
		statusPill := fmt.Sprintf("%s%s%s ON%s", colorBold, colorGreen, symBullet, colorReset)
		if !en {
			statusPill = fmt.Sprintf("%s%s OFF%s", colorDim, symCircle, colorReset)
		}
		inheritHint := ""
		if p.CodexEnabled == nil && p.CodexMode == "" && p.CodexContent == "" {
			inheritHint = fmt.Sprintf("  %s(all inherited from Default)%s", colorDim, colorReset)
		}
		fmt.Fprintf(a.stdout, "  %s%s%s Codex CLI notifications%s%s\n",
			colorDim, symBar, " ", colorReset, inheritHint)
		fmt.Fprintf(a.stdout, "  %s%s%s %s  mode:%s%s%s  content:%s%s%s\n",
			colorDim, symBar, colorReset,
			statusPill,
			colorBold, mode, colorReset,
			colorBold, content, colorReset)
	case tabClaude:
		en, mode, content := p.ToolPrefs("claude")
		statusPill := fmt.Sprintf("%s%s%s ON%s", colorBold, colorGreen, symBullet, colorReset)
		if !en {
			statusPill = fmt.Sprintf("%s%s OFF%s", colorDim, symCircle, colorReset)
		}
		inheritHint := ""
		if p.ClaudeEnabled == nil && p.ClaudeMode == "" && p.ClaudeContent == "" {
			inheritHint = fmt.Sprintf("  %s(all inherited from Default)%s", colorDim, colorReset)
		}
		fmt.Fprintf(a.stdout, "  %s%s%s Claude Code notifications%s%s\n",
			colorDim, symBar, " ", colorReset, inheritHint)
		fmt.Fprintf(a.stdout, "  %s%s%s %s  mode:%s%s%s  content:%s%s%s\n",
			colorDim, symBar, colorReset,
			statusPill,
			colorBold, mode, colorReset,
			colorBold, content, colorReset)
	}
	fmt.Fprintln(a.stdout)
}

func (a *App) tabMenuItems(tab int, p Preferences) []menuItem {
	switch tab {
	case tabDefault:
		return a.defaultTabItems(p)
	case tabCodex:
		return a.codexTabItems(p)
	case tabClaude:
		return a.claudeTabItems(p)
	}
	return nil
}

func (a *App) defaultTabItems(p Preferences) []menuItem {
	modeOpts := []string{"auto  " + colorDim + symDot + " toast first, popup fallback" + colorReset,
		"toast " + colorDim + symDot + " Windows system notification" + colorReset,
		"popup " + colorDim + symDot + " popup dialog" + colorReset}
	contentOpts := []string{
		"summary  " + colorDim + symDot + " short summary" + colorReset,
		"full     " + colorDim + symDot + " full assistant message" + colorReset,
		"complete " + colorDim + symDot + " minimal text" + colorReset,
	}
	return []menuItem{
		{
			label: fmt.Sprintf("%s Toggle notifications     %s", symSpark, toggleIndicator(p.Enabled)),
			action: func(prefs *Preferences) actionResult {
				prefs.Enabled = !prefs.Enabled
				return actionResult{status: a.saveOrSessionText(*prefs)}
			},
		},
		{
			label: fmt.Sprintf("%s Notification mode        %s%s%s", symBell, colorDim, p.Mode, colorReset),
			action: func(prefs *Preferences) actionResult {
				start := indexOf([]string{"auto", "toast", "popup"}, prefs.Mode)
				sel, err := a.selectSingleTTY("Default Mode", "Notification delivery method for all tools.", modeOpts, start)
				if err != nil {
					return actionResult{status: fmt.Sprintf("%s✗ %v%s", colorRed, err, colorReset)}
				}
				prefs.Mode = []string{"auto", "toast", "popup"}[sel]
				return actionResult{status: a.saveOrSessionText(*prefs)}
			},
		},
		{
			label: fmt.Sprintf("%s Content mode             %s%s%s", symGear, colorDim, p.Content, colorReset),
			action: func(prefs *Preferences) actionResult {
				start := indexOf([]string{"summary", "full", "complete"}, prefs.Content)
				sel, err := a.selectSingleTTY("Default Content", "What to show in the notification body.", contentOpts, start)
				if err != nil {
					return actionResult{status: fmt.Sprintf("%s✗ %v%s", colorRed, err, colorReset)}
				}
				prefs.Content = []string{"summary", "full", "complete"}[sel]
				return actionResult{status: a.saveOrSessionText(*prefs)}
			},
		},
		{
			label: fmt.Sprintf("%s Configure extra fields", symDot),
			action: func(prefs *Preferences) actionResult {
				opts := []string{"Include project directory", "Include model name", "Include event type"}
				cur := map[int]bool{0: prefs.IncludeDir, 1: prefs.IncludeModel, 2: prefs.IncludeEvent}
				sel, err := a.selectMultiTTY("Extra Fields", "Toggle additional info in notifications.", opts, cur)
				if err != nil {
					return actionResult{status: fmt.Sprintf("%s✗ %v%s", colorRed, err, colorReset)}
				}
				prefs.IncludeDir = sel[0]
				prefs.IncludeModel = sel[1]
				prefs.IncludeEvent = sel[2]
				prefs.FieldsConfigured = true
				return actionResult{status: a.saveOrSessionText(*prefs)}
			},
		},
		{
			label: fmt.Sprintf("%s Toast AppId              %s%s%s", symPlug, colorDim, p.ToastAppID, colorReset),
			action: func(prefs *Preferences) actionResult {
				appID, err := a.promptLine("  Toast AppId (blank = default): ")
				if err != nil {
					return actionResult{status: fmt.Sprintf("%s✗ %v%s", colorRed, err, colorReset)}
				}
				if appID == "" {
					appID = defaultToastAppID
				}
				prefs.ToastAppID = appID
				return actionResult{status: a.saveOrSessionText(*prefs)}
			},
		},
		{label: fmt.Sprintf("%s%s%s", colorDim, strings.Repeat(symHLine, 40), colorReset)},
		{
			label: fmt.Sprintf("%s Send preview notification", symBell),
			action: func(prefs *Preferences) actionResult {
				if err := a.previewNotification(*prefs); err != nil {
					return actionResult{status: fmt.Sprintf("%s%s✗ Preview failed:%s %v", colorBold, colorRed, colorReset, err)}
				}
				return actionResult{status: fmt.Sprintf("%s%s✓ Preview sent.%s", colorBold, colorGreen, colorReset)}
			},
		},
		{
			label: fmt.Sprintf("%s Save settings now", symDisk),
			action: func(prefs *Preferences) actionResult {
				if err := a.savePreferences(*prefs); err != nil {
					return actionResult{status: fmt.Sprintf("%s%s✗ Save failed:%s %v", colorBold, colorRed, colorReset, err)}
				}
				return actionResult{status: fmt.Sprintf("%s%s✓ Saved.%s", colorBold, colorGreen, colorReset)}
			},
		},
		{
			label: fmt.Sprintf("%s Exit", symWave),
			action: func(prefs *Preferences) actionResult {
				_ = a.savePreferences(*prefs)
				clearScreen(a.stdout)
				fmt.Fprintf(a.stdout, "  %s%sGoodbye!%s\n\n", colorDim, colorCyan, colorReset)
				return actionExit
			},
		},
	}
}

func (a *App) toolModeAction(toolName string, modePtr *string) func(prefs *Preferences) actionResult {
	modeLabels := []string{
		colorDim + "global " + symDot + " use Default setting" + colorReset,
		"auto  " + colorDim + symDot + " toast first, popup fallback" + colorReset,
		"toast " + colorDim + symDot + " Windows system notification" + colorReset,
		"popup " + colorDim + symDot + " popup dialog" + colorReset,
	}
	modeValues := []string{"auto", "toast", "popup"}

	return func(prefs *Preferences) actionResult {
		cur := 0
		if *modePtr != "" {
			cur = indexOf(modeValues, *modePtr) + 1
		}
		sel, err := a.selectSingleTTY(toolName+" Mode", "Choose mode or 'global' to inherit Default.", modeLabels, cur)
		if err != nil {
			return actionResult{status: fmt.Sprintf("%s✗ %v%s", colorRed, err, colorReset)}
		}
		if sel == 0 {
			*modePtr = ""
		} else {
			*modePtr = modeValues[sel-1]
		}
		return actionResult{status: a.saveOrSessionText(*prefs)}
	}
}

func (a *App) toolContentAction(toolName string, contentPtr *string) func(prefs *Preferences) actionResult {
	contentLabels := []string{
		colorDim + "global " + symDot + " use Default setting" + colorReset,
		"summary  " + colorDim + symDot + " short summary" + colorReset,
		"full     " + colorDim + symDot + " full assistant message" + colorReset,
		"complete " + colorDim + symDot + " minimal text" + colorReset,
	}
	contentValues := []string{"summary", "full", "complete"}

	return func(prefs *Preferences) actionResult {
		cur := 0
		if *contentPtr != "" {
			cur = indexOf(contentValues, *contentPtr) + 1
		}
		sel, err := a.selectSingleTTY(toolName+" Content", "Choose content mode or 'global' to inherit Default.", contentLabels, cur)
		if err != nil {
			return actionResult{status: fmt.Sprintf("%s✗ %v%s", colorRed, err, colorReset)}
		}
		if sel == 0 {
			*contentPtr = ""
		} else {
			*contentPtr = contentValues[sel-1]
		}
		return actionResult{status: a.saveOrSessionText(*prefs)}
	}
}

func toolEnabledLabel(ptr *bool, globalEnabled bool) string {
	if ptr == nil {
		if globalEnabled {
			return fmt.Sprintf("%sinherit%s %s%s%s ON%s", colorDim, colorReset, colorDim, symDot, colorGreen, colorReset)
		}
		return fmt.Sprintf("%sinherit%s %s%s%s OFF%s", colorDim, colorReset, colorDim, symDot, colorRed, colorReset)
	}
	if *ptr {
		return fmt.Sprintf("%s%s%s ON%s", colorBold, colorGreen, symBullet, colorReset)
	}
	return fmt.Sprintf("%s%s OFF%s", colorDim, symCircle, colorReset)
}

func toolOverrideHint(val string) string {
	if val == "" {
		return fmt.Sprintf("%sinherit%s", colorDim, colorReset)
	}
	return fmt.Sprintf("%s%s%s", colorCyan, val, colorReset)
}

func (a *App) codexTabItems(p Preferences) []menuItem {
	return []menuItem{
		{
			label: fmt.Sprintf("%s Toggle Codex             %s", symSpark, toolEnabledLabel(p.CodexEnabled, p.Enabled)),
			action: func(prefs *Preferences) actionResult {
				if prefs.CodexEnabled == nil {
					prefs.CodexEnabled = boolPtr(false)
				} else if !*prefs.CodexEnabled {
					prefs.CodexEnabled = boolPtr(true)
				} else {
					prefs.CodexEnabled = nil
				}
				return actionResult{status: a.saveOrSessionText(*prefs)}
			},
		},
		{
			label: fmt.Sprintf("%s Codex mode               %s", symBell, toolOverrideHint(p.CodexMode)),
			action: func(prefs *Preferences) actionResult {
				return a.toolModeAction("Codex", &prefs.CodexMode)(prefs)
			},
		},
		{
			label: fmt.Sprintf("%s Codex content            %s", symGear, toolOverrideHint(p.CodexContent)),
			action: func(prefs *Preferences) actionResult {
				return a.toolContentAction("Codex", &prefs.CodexContent)(prefs)
			},
		},
		{label: fmt.Sprintf("%s%s%s", colorDim, strings.Repeat(symHLine, 40), colorReset)},
		{
			label: fmt.Sprintf("%s Install Codex hook       %s~/.codex/config.toml%s", symPlug, colorDim, colorReset),
			action: func(prefs *Preferences) actionResult {
				if err := a.runInstall([]string{"codex"}); err != nil {
					return actionResult{status: fmt.Sprintf("%s%s✗ Install failed:%s %v", colorBold, colorRed, colorReset, err)}
				}
				return actionResult{status: fmt.Sprintf("%s%s✓ Codex hook installed.%s", colorBold, colorGreen, colorReset)}
			},
		},
		{
			label: fmt.Sprintf("%s Send Codex preview", symBell),
			action: func(prefs *Preferences) actionResult {
				_, mode, content := prefs.ToolPrefs("codex")
				if err := a.previewWithOverrides(*prefs, mode, content); err != nil {
					return actionResult{status: fmt.Sprintf("%s%s✗ Preview failed:%s %v", colorBold, colorRed, colorReset, err)}
				}
				return actionResult{status: fmt.Sprintf("%s%s✓ Codex preview sent.%s", colorBold, colorGreen, colorReset)}
			},
		},
	}
}

func (a *App) claudeTabItems(p Preferences) []menuItem {
	return []menuItem{
		{
			label: fmt.Sprintf("%s Toggle Claude            %s", symSpark, toolEnabledLabel(p.ClaudeEnabled, p.Enabled)),
			action: func(prefs *Preferences) actionResult {
				if prefs.ClaudeEnabled == nil {
					prefs.ClaudeEnabled = boolPtr(false)
				} else if !*prefs.ClaudeEnabled {
					prefs.ClaudeEnabled = boolPtr(true)
				} else {
					prefs.ClaudeEnabled = nil
				}
				return actionResult{status: a.saveOrSessionText(*prefs)}
			},
		},
		{
			label: fmt.Sprintf("%s Claude mode              %s", symBell, toolOverrideHint(p.ClaudeMode)),
			action: func(prefs *Preferences) actionResult {
				return a.toolModeAction("Claude", &prefs.ClaudeMode)(prefs)
			},
		},
		{
			label: fmt.Sprintf("%s Claude content           %s", symGear, toolOverrideHint(p.ClaudeContent)),
			action: func(prefs *Preferences) actionResult {
				return a.toolContentAction("Claude", &prefs.ClaudeContent)(prefs)
			},
		},
		{label: fmt.Sprintf("%s%s%s", colorDim, strings.Repeat(symHLine, 40), colorReset)},
		{
			label: fmt.Sprintf("%s Install Claude hook      %s~/.claude/settings.json%s", symPlug, colorDim, colorReset),
			action: func(prefs *Preferences) actionResult {
				if err := a.runInstall([]string{"claude"}); err != nil {
					return actionResult{status: fmt.Sprintf("%s%s✗ Install failed:%s %v", colorBold, colorRed, colorReset, err)}
				}
				return actionResult{status: fmt.Sprintf("%s%s✓ Claude hook installed.%s", colorBold, colorGreen, colorReset)}
			},
		},
		{
			label: fmt.Sprintf("%s Send Claude preview", symBell),
			action: func(prefs *Preferences) actionResult {
				_, mode, content := prefs.ToolPrefs("claude")
				if err := a.previewWithOverrides(*prefs, mode, content); err != nil {
					return actionResult{status: fmt.Sprintf("%s%s✗ Preview failed:%s %v", colorBold, colorRed, colorReset, err)}
				}
				return actionResult{status: fmt.Sprintf("%s%s✓ Claude preview sent.%s", colorBold, colorGreen, colorReset)}
			},
		},
	}
}

func (a *App) previewWithOverrides(p Preferences, mode, content string) error {
	title, body, _ := event.RenderNotificationWithOptions(event.Payload{
		Type:                 "agent-turn-complete",
		Summary:              "Sample summary: work completed.",
		LastAssistantMessage: "Sample full answer: all requested changes are finished.",
		CWD:                  "C:\\sample\\project",
		Model:                "gpt-5",
	}, event.RenderOptions{
		ContentMode:  event.ContentMode(content),
		IncludeDir:   p.IncludeDir,
		IncludeModel: p.IncludeModel,
		IncludeEvent: p.IncludeEvent,
	})

	service := notifier.NewWithConfig(notifier.Config{
		Mode:       mode,
		ToastAppID: p.ToastAppID,
	})
	return service.Notify(title, body)
}

func (a *App) renderHeader() {
	fmt.Fprintln(a.stdout)
	fmt.Fprintf(a.stdout, "  %s%s╭─ %s⚡ cc-notify%s %s%s %s─╮%s\n",
		colorDim, colorMagenta, colorBold+colorCyan, colorReset,
		colorDim, version,
		colorMagenta, colorReset)
	fmt.Fprintf(a.stdout, "  %s%s╰─ %sNotifications for Codex CLI & Claude Code%s %s─╯%s\n",
		colorDim, colorMagenta, colorCyan, colorReset,
		colorMagenta, colorReset)
	fmt.Fprintln(a.stdout)
}

func toggleIndicator(on bool) string {
	if on {
		return fmt.Sprintf("%s%s%s ON%s", colorBold, colorGreen, symBullet, colorReset)
	}
	return fmt.Sprintf("%s%s OFF%s", colorDim, symCircle, colorReset)
}

func (a *App) runInteractiveLineUI(prefs *Preferences) error {
	reader := bufio.NewReader(a.stdin)

	for {
		a.renderInteractiveMenu(*prefs)
		fmt.Fprintf(a.stdout, "\n  %s%s❯%s ", colorBold, colorCyan, colorReset)

		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			return fmt.Errorf("read interactive input: %w", readErr)
		}
		choice := strings.TrimSpace(line)

		switch choice {
		case "1":
			prefs.Enabled = !prefs.Enabled
			a.printSavedOrSession(*prefs)
		case "2":
			prefs.Mode = nextMode(prefs.Mode)
			a.printSavedOrSession(*prefs)
			fmt.Fprintf(a.stdout, "  Mode -> %s%s%s (%s)\n", colorCyan, prefs.Mode, colorReset, modeHint(prefs.Mode))
		case "3":
			prefs.Content = nextContentMode(prefs.Content)
			a.printSavedOrSession(*prefs)
		case "4":
			fmt.Fprintf(a.stdout, "  Toast AppId (blank = default): ")
			appID, e := readInteractiveLine(reader, nil)
			if e != nil {
				return fmt.Errorf("read app id: %w", e)
			}
			appID = strings.TrimSpace(appID)
			if appID == "" {
				appID = defaultToastAppID
			}
			prefs.ToastAppID = appID
			a.printSavedOrSession(*prefs)
		case "5":
			if err := a.previewNotification(*prefs); err != nil {
				fmt.Fprintf(a.stderr, "  %s%s✗%s preview failed: %v\n", colorBold, colorRed, colorReset, err)
			} else {
				fmt.Fprintf(a.stdout, "  %s%s✓%s Preview sent.\n", colorBold, colorGreen, colorReset)
			}
		case "6":
			if err := a.runInstall([]string{"codex"}); err != nil {
				fmt.Fprintf(a.stderr, "  %s%s✗%s Codex install failed: %v\n", colorBold, colorRed, colorReset, err)
			} else {
				fmt.Fprintf(a.stdout, "  %s%s✓%s Codex hook installed.\n", colorBold, colorGreen, colorReset)
			}
		case "7":
			if err := a.runInstall([]string{"claude"}); err != nil {
				fmt.Fprintf(a.stderr, "  %s%s✗%s Claude install failed: %v\n", colorBold, colorRed, colorReset, err)
			} else {
				fmt.Fprintf(a.stdout, "  %s%s✓%s Claude Code hook installed.\n", colorBold, colorGreen, colorReset)
			}
		case "8":
			if err := a.savePreferences(*prefs); err != nil {
				fmt.Fprintf(a.stderr, "  %s%s✗%s save failed: %v\n", colorBold, colorRed, colorReset, err)
			} else {
				fmt.Fprintf(a.stdout, "  %s%s✓ Saved.%s\n", colorBold, colorGreen, colorReset)
			}
		case "0", "q", "quit", "exit":
			if err := a.savePreferences(*prefs); err != nil {
				fmt.Fprintf(a.stderr, "  %snote:%s save on exit failed: %v\n", colorYellow, colorReset, err)
			}
			fmt.Fprintf(a.stdout, "\n  %s%sGoodbye!%s\n\n", colorDim, colorCyan, colorReset)
			return nil
		default:
			fmt.Fprintf(a.stderr, "  %sUnknown option. Choose 1-8 or 0 to exit.%s\n", colorDim, colorReset)
		}
	}
}

func (a *App) renderInteractiveMenu(p Preferences) {
	fmt.Fprintln(a.stdout)
	a.renderHeader()

	// Simple status for line-based UI
	statusStr := "ON"
	if !p.Enabled {
		statusStr = "OFF"
	}
	fmt.Fprintf(a.stdout, "  Status: %s  Mode: %s  Content: %s\n", statusStr, p.Mode, p.Content)
	fmt.Fprintf(a.stdout, "  %s%sSettings auto-saved to disk on every change.%s\n", colorDim, symDot+" ", colorReset)
	fmt.Fprintln(a.stdout)

	type menuSection struct {
		label string
		items []struct {
			key  string
			text string
			hint string
		}
	}

	sections := []menuSection{
		{
			label: "Settings",
			items: []struct {
				key  string
				text string
				hint string
			}{
				{"1", "Toggle notifications", ""},
				{"2", "Cycle notification mode", "auto/toast/popup"},
				{"3", "Cycle content mode", "summary/full/complete"},
				{"4", "Set Toast AppId", ""},
			},
		},
		{
			label: "Actions",
			items: []struct {
				key  string
				text string
				hint string
			}{
				{"5", "Send preview notification", ""},
				{"6", "Install Codex hook", "~/.codex/config.toml"},
				{"7", "Install Claude Code hook", "~/.claude/settings.json"},
				{"8", "Save settings now", ""},
				{"0", "Exit", ""},
			},
		},
	}

	for _, section := range sections {
		fmt.Fprintf(a.stdout, "  %s%s%s %s%s\n", colorMagenta, symBar, colorReset, colorBold+section.label, colorReset)
		for _, item := range section.items {
			hint := ""
			if item.hint != "" {
				hint = fmt.Sprintf("  %s%s%s", colorDim, item.hint, colorReset)
			}
			fmt.Fprintf(a.stdout, "  %s%s%s %s%s%s%s  %s%s\n",
				colorMagenta, symBar, colorReset,
				colorCyan, item.key, colorReset,
				colorDim+")"+colorReset,
				item.text, hint)
		}
		fmt.Fprintln(a.stdout)
	}
}

func (a *App) selectSingleTTY(title, status string, options []string, cursor int) (int, error) {
	restore, ok := enableRawInput(a.stdin, a.stdout)
	if !ok {
		return -1, fmt.Errorf("raw input unavailable")
	}
	defer restore()

	if cursor < 0 || cursor >= len(options) {
		cursor = 0
	}
	reader := bufio.NewReader(a.stdin)

	for {
		clearScreen(a.stdout)
		fmt.Fprintln(a.stdout)
		fmt.Fprintf(a.stdout, "  %s%s%s %s%s\n", colorMagenta, symBar, colorReset, colorBold+colorCyan+title, colorReset)
		fmt.Fprintf(a.stdout, "  %s%s%s %s%s%s\n", colorMagenta, symBar, colorReset, colorDim, status, colorReset)
		fmt.Fprintf(a.stdout, "  %s%s%s\n", colorMagenta, symBar, colorReset)

		for i, option := range options {
			radio := fmt.Sprintf("%s%s%s", colorDim, symRadioOff, colorReset)
			if i == cursor {
				radio = fmt.Sprintf("%s%s%s", colorCyan, symRadioOn, colorReset)
			}
			if i == cursor {
				fmt.Fprintf(a.stdout, "  %s%s%s %s %s%s%s\n", colorMagenta, symBar, colorReset, radio, colorBold, option, colorReset)
			} else {
				fmt.Fprintf(a.stdout, "  %s%s%s %s %s%s%s\n", colorMagenta, symBar, colorReset, radio, colorDim, option, colorReset)
			}
		}

		fmt.Fprintln(a.stdout)
		fmt.Fprintf(a.stdout, "  %s%s%s\n", colorDim, strings.Repeat(symHLine, 50), colorReset)
		fmt.Fprintf(a.stdout, "  %s↑/↓%s navigate  %s⏎%s confirm  %sesc%s back\n",
			colorBold, colorReset,
			colorBold, colorReset,
			colorBold, colorReset)

		key, err := readKey(reader)
		if err != nil {
			return -1, err
		}
		switch key {
		case keyUp:
			if cursor > 0 {
				cursor--
			}
		case keyDown:
			if cursor < len(options)-1 {
				cursor++
			}
		case keyEnter:
			return cursor, nil
		case keyEsc:
			return len(options) - 1, nil
		}
	}
}

func (a *App) selectMultiTTY(title, status string, options []string, selected map[int]bool) (map[int]bool, error) {
	restore, ok := enableRawInput(a.stdin, a.stdout)
	if !ok {
		return selected, nil
	}
	defer restore()

	cursor := 0
	reader := bufio.NewReader(a.stdin)

	for {
		clearScreen(a.stdout)
		fmt.Fprintln(a.stdout)
		fmt.Fprintf(a.stdout, "  %s%s%s %s%s\n", colorMagenta, symBar, colorReset, colorBold+colorCyan+title, colorReset)
		fmt.Fprintf(a.stdout, "  %s%s%s %s%s%s\n", colorMagenta, symBar, colorReset, colorDim, status, colorReset)
		fmt.Fprintf(a.stdout, "  %s%s%s\n", colorMagenta, symBar, colorReset)

		for i, option := range options {
			mark := fmt.Sprintf("%s%s%s", colorDim, symUncheck, colorReset)
			if selected[i] {
				mark = fmt.Sprintf("%s%s%s", colorGreen, symCheckBox, colorReset)
			}
			if i == cursor {
				fmt.Fprintf(a.stdout, "  %s%s%s %s%s%s %s %s%s%s\n", colorMagenta, symBar, colorReset,
					colorCyan, symArrow, colorReset, mark, colorBold, option, colorReset)
			} else {
				fmt.Fprintf(a.stdout, "  %s%s%s   %s %s\n", colorMagenta, symBar, colorReset, mark, option)
			}
		}

		fmt.Fprintln(a.stdout)
		fmt.Fprintf(a.stdout, "  %s%s%s\n", colorDim, strings.Repeat(symHLine, 50), colorReset)
		fmt.Fprintf(a.stdout, "  %s↑/↓%s navigate  %sspace%s toggle  %s⏎%s confirm\n",
			colorBold, colorReset,
			colorBold, colorReset,
			colorBold, colorReset)

		key, err := readKey(reader)
		if err != nil {
			return selected, err
		}
		switch key {
		case keyUp:
			if cursor > 0 {
				cursor--
			}
		case keyDown:
			if cursor < len(options)-1 {
				cursor++
			}
		case keySpace:
			selected[cursor] = !selected[cursor]
		case keyEnter:
			return selected, nil
		}
	}
}

func (a *App) promptLine(prompt string) (string, error) {
	fmt.Fprintln(a.stdout)
	fmt.Fprint(a.stdout, colorCyan+prompt+colorReset)
	reader := bufio.NewReader(a.stdin)
	line, err := readInteractiveLine(reader, a.stdout)
	if err != nil {
		return "", fmt.Errorf("read line: %w", err)
	}
	return strings.TrimSpace(line), nil
}

func readInteractiveLine(reader *bufio.Reader, echo io.Writer) (string, error) {
	var chars []rune
	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) && len(chars) > 0 {
				if echo != nil {
					fmt.Fprintln(echo)
				}
				return string(chars), nil
			}
			return "", err
		}

		switch r {
		case '\r':
			if reader.Buffered() > 0 {
				if next, peekErr := reader.Peek(1); peekErr == nil && len(next) == 1 && next[0] == '\n' {
					_, _ = reader.ReadByte()
				}
			}
			if echo != nil {
				fmt.Fprintln(echo)
			}
			return string(chars), nil
		case '\n':
			if echo != nil {
				fmt.Fprintln(echo)
			}
			return string(chars), nil
		case '\b', 127:
			if len(chars) > 0 {
				chars = chars[:len(chars)-1]
				if echo != nil {
					fmt.Fprint(echo, "\b \b")
				}
			}
		default:
			if r < 32 {
				continue
			}
			chars = append(chars, r)
			if echo != nil {
				fmt.Fprint(echo, string(r))
			}
		}
	}
}

func boolPtr(v bool) *bool { return &v }

func (a *App) saveOrSessionText(p Preferences) string {
	if err := a.savePreferences(p); err != nil {
		return fmt.Sprintf("%s%s✗ Save failed:%s %v", colorBold, colorRed, colorReset, err)
	}
	return fmt.Sprintf("%s%s✓ Saved.%s", colorBold, colorGreen, colorReset)
}

func (a *App) printSavedOrSession(p Preferences) {
	msg := a.saveOrSessionText(p)
	fmt.Fprintf(a.stdout, "  %s\n", msg)
}

func (a *App) previewNotification(p Preferences) error {
	title, body, _ := event.RenderNotificationWithOptions(event.Payload{
		Type:                 "agent-turn-complete",
		Summary:              "Sample summary: work completed.",
		LastAssistantMessage: "Sample full answer: all requested changes are finished.",
		CWD:                  "C:\\sample\\project",
		Model:                "gpt-5",
	}, event.RenderOptions{
		ContentMode:  event.ContentMode(p.Content),
		IncludeDir:   p.IncludeDir,
		IncludeModel: p.IncludeModel,
		IncludeEvent: p.IncludeEvent,
	})

	service := notifier.NewWithConfig(notifier.Config{
		Mode:       p.Mode,
		ToastAppID: p.ToastAppID,
	})
	return service.Notify(title, body)
}

func nextMode(current string) string {
	switch current {
	case "auto":
		return "toast"
	case "toast":
		return "popup"
	default:
		return "auto"
	}
}

func modeHint(mode string) string {
	switch mode {
	case "toast":
		return "Windows system notification"
	case "popup":
		return "popup dialog"
	default:
		return "toast first, popup fallback"
	}
}

func nextContentMode(current string) string {
	switch current {
	case "summary":
		return "full"
	case "full":
		return "complete"
	default:
		return "summary"
	}
}

func readKey(reader *bufio.Reader) (keyCode, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return keyUnknown, err
	}
	switch b {
	case 13, 10:
		return keyEnter, nil
	case ' ':
		return keySpace, nil
	case 'k', 'K':
		return keyUp, nil
	case 'j', 'J':
		return keyDown, nil
	case 27:
		b2, e2 := reader.ReadByte()
		if e2 != nil {
			return keyEsc, nil
		}
		if b2 == '[' {
			b3, e3 := reader.ReadByte()
			if e3 != nil {
				return keyEsc, nil
			}
			switch b3 {
			case 'A':
				return keyUp, nil
			case 'B':
				return keyDown, nil
			case 'C':
				return keyRight, nil
			case 'D':
				return keyLeft, nil
			default:
				// 其他 CSI 序列（如 Home/End/PgUp/PgDn 等），忽略
				return keyUnknown, nil
			}
		}
		// 单独的 ESC 键（没有后续 [）
		return keyEsc, nil
	case 0, 224:
		b2, e2 := reader.ReadByte()
		if e2 != nil {
			return keyUnknown, nil
		}
		if b2 == 72 {
			return keyUp, nil
		}
		if b2 == 80 {
			return keyDown, nil
		}
		return keyUnknown, nil
	default:
		return keyUnknown, nil
	}
}

func clearScreen(out io.Writer) {
	//nolint:errcheck
	fmt.Fprint(out, "\x1b[2J\x1b[H")
}

func (a *App) stdinIsTTY() bool {
	f, ok := a.stdin.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (a *App) stdoutIsTTY() bool {
	f, ok := a.stdout.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func indexOf(options []string, current string) int {
	for i, item := range options {
		if item == current {
			return i
		}
	}
	return 0
}

func (a *App) previewModeChoice(p Preferences) error {
	service := notifier.NewWithConfig(notifier.Config{
		Mode:       p.Mode,
		ToastAppID: p.ToastAppID,
	})
	return service.Notify("Notification Mode Selected", "Mode: "+p.Mode+" ("+modeHint(p.Mode)+")")
}
