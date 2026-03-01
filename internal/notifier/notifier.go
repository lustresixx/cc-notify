package notifier

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf16"
)

// Service sends user-facing notifications.
type Service interface {
	Notify(title, body string) error
}

// Action is a clickable item attached to a notification.
type Action struct {
	Label string
	URI   string
}

// ActionService supports actionable notifications with buttons.
type ActionService interface {
	Service
	NotifyWithActions(title, body string, actions []Action) error
}

func buildToastScript(title, body, appID string) string {
	return buildToastScriptWithActions(title, body, appID, nil)
}

func buildToastScriptWithActions(title, body, appID string, actions []Action) string {
	titleB64 := base64.StdEncoding.EncodeToString([]byte(title))
	bodyB64 := base64.StdEncoding.EncodeToString([]byte(body))
	appIDB64 := base64.StdEncoding.EncodeToString([]byte(appID))
	labelArray := base64ArrayFromActions(actions, func(a Action) string { return a.Label })
	uriArray := base64ArrayFromActions(actions, func(a Action) string { return a.URI })

	return fmt.Sprintf(
		`$ErrorActionPreference = 'Stop'
$null = [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]
$null = [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime]
$title = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$body = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$appId = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$actionLabels = %s
$actionUris = %s
$xmlContent = "<toast><visual><binding template='ToastGeneric'><text></text><text></text></binding></visual></toast>"
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($xmlContent)
$textNodes = $xml.GetElementsByTagName('text')
$null = $textNodes.Item(0).AppendChild($xml.CreateTextNode($title))
$null = $textNodes.Item(1).AppendChild($xml.CreateTextNode($body))
if ($actionLabels.Count -eq $actionUris.Count -and $actionLabels.Count -gt 0) {
  $actionsNode = $xml.CreateElement('actions')
  for ($i = 0; $i -lt $actionLabels.Count; $i++) {
    $label = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($actionLabels[$i]))
    $uri = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($actionUris[$i]))
    if ([string]::IsNullOrWhiteSpace($label) -or [string]::IsNullOrWhiteSpace($uri)) {
      continue
    }
    $actionNode = $xml.CreateElement('action')
    $null = $actionNode.SetAttribute('content', $label)
    $null = $actionNode.SetAttribute('activationType', 'protocol')
    $null = $actionNode.SetAttribute('arguments', $uri)
    $null = $actionsNode.AppendChild($actionNode)
  }
  if ($actionsNode.HasChildNodes) {
    $null = $xml.DocumentElement.AppendChild($actionsNode)
  }
}
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($appId).Show($toast)
`,
		titleB64,
		bodyB64,
		appIDB64,
		labelArray,
		uriArray,
	)
}

func buildPopupScript(title, body string) string {
	titleB64 := base64.StdEncoding.EncodeToString([]byte(title))
	bodyB64 := base64.StdEncoding.EncodeToString([]byte(body))

	return fmt.Sprintf(
		`$ErrorActionPreference = 'Stop'
$title = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$body = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$wshell = New-Object -ComObject WScript.Shell
$null = $wshell.Popup($body, 8, $title, 0x40)
`,
		titleB64,
		bodyB64,
	)
}

func encodePowerShellCommand(command string) string {
	utf16Text := utf16.Encode([]rune(command))
	utf16LEBytes := make([]byte, len(utf16Text)*2)
	for i, code := range utf16Text {
		utf16LEBytes[i*2] = byte(code)
		utf16LEBytes[i*2+1] = byte(code >> 8)
	}
	return base64.StdEncoding.EncodeToString(utf16LEBytes)
}

func base64ArrayFromActions(actions []Action, selector func(Action) string) string {
	if len(actions) == 0 {
		return "@()"
	}
	values := make([]string, 0, len(actions))
	for _, action := range actions {
		raw := selector(action)
		values = append(values, base64.StdEncoding.EncodeToString([]byte(raw)))
	}
	quoted := make([]string, 0, len(values))
	for _, item := range values {
		quoted = append(quoted, "'"+item+"'")
	}
	return "@(" + strings.Join(quoted, ",") + ")"
}
