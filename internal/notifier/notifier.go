package notifier

import (
	"encoding/base64"
	"fmt"
	"unicode/utf16"
)

// Service sends user-facing notifications.
type Service interface {
	Notify(title, body string) error
}

func buildToastScript(title, body, appID string) string {
	titleB64 := base64.StdEncoding.EncodeToString([]byte(title))
	bodyB64 := base64.StdEncoding.EncodeToString([]byte(body))
	appIDB64 := base64.StdEncoding.EncodeToString([]byte(appID))

	return fmt.Sprintf(
		`$ErrorActionPreference = 'Stop'
$null = [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]
$null = [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime]
$title = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$body = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$appId = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$xmlContent = "<toast><visual><binding template='ToastGeneric'><text></text><text></text></binding></visual></toast>"
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($xmlContent)
$textNodes = $xml.GetElementsByTagName('text')
$null = $textNodes.Item(0).AppendChild($xml.CreateTextNode($title))
$null = $textNodes.Item(1).AppendChild($xml.CreateTextNode($body))
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($appId).Show($toast)
`,
		titleB64,
		bodyB64,
		appIDB64,
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
