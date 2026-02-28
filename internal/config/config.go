package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPath returns Codex config path in the user's home directory.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

// UpsertNotify inserts or replaces a top-level notify assignment.
func UpsertNotify(content string, command []string) (string, bool, error) {
	if len(command) == 0 {
		return "", false, errors.New("notify command cannot be empty")
	}

	bom, content := stripBOM(content)
	newline := detectNewline(content)
	lines := splitLines(content)
	notifyLine := renderNotifyLine(command)

	if len(lines) == 0 {
		return bom + notifyLine + newline, true, nil
	}

	firstTable := firstTableIndex(lines)
	start, end, found, err := findTopLevelNotify(lines[:firstTable])
	if err != nil {
		return "", false, err
	}

	if found {
		if end-start == 1 && strings.TrimSpace(lines[start]) == notifyLine {
			return bom + content, false, nil
		}

		updated := append([]string{}, lines...)
		updated = append(updated[:start], append([]string{notifyLine}, updated[end:]...)...)
		return bom + joinLines(updated, newline), true, nil
	}

	if firstTable == 0 && len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "[") {
		lines = append([]string{notifyLine, ""}, lines...)
		return bom + joinLines(lines, newline), true, nil
	}

	lines = append(lines[:firstTable], append([]string{notifyLine}, lines[firstTable:]...)...)
	return bom + joinLines(lines, newline), true, nil
}

// RemoveNotify removes a top-level notify assignment if it exists.
func RemoveNotify(content string) (string, bool, error) {
	bom, content := stripBOM(content)
	newline := detectNewline(content)
	lines := splitLines(content)
	if len(lines) == 0 {
		return bom + content, false, nil
	}

	firstTable := firstTableIndex(lines)
	start, end, found, err := findTopLevelNotify(lines[:firstTable])
	if err != nil {
		return "", false, err
	}
	if !found {
		return bom + content, false, nil
	}

	updated := append([]string{}, lines...)
	updated = append(updated[:start], updated[end:]...)
	updated = trimLeadingBlankLines(updated)
	return bom + joinLines(updated, newline), true, nil
}

// stripBOM removes a leading UTF-8 BOM if present, returning the BOM
// string and the remaining content separately so callers can re-prepend it.
func stripBOM(content string) (string, string) {
	const bom = "\xEF\xBB\xBF"
	if strings.HasPrefix(content, bom) {
		return bom, content[len(bom):]
	}
	// Also handle the decoded Unicode BOM codepoint.
	if strings.HasPrefix(content, "\uFEFF") {
		return "\uFEFF", strings.TrimPrefix(content, "\uFEFF")
	}
	return "", content
}

func detectNewline(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func splitLines(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.TrimSuffix(normalized, "\n")
	if normalized == "" {
		return nil
	}
	return strings.Split(normalized, "\n")
}

func joinLines(lines []string, newline string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, newline) + newline
}

func firstTableIndex(lines []string) int {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			return i
		}
	}
	return len(lines)
}

func findTopLevelNotify(rootLines []string) (start int, end int, found bool, err error) {
	for i := 0; i < len(rootLines); i++ {
		if !isNotifyAssignmentStart(rootLines[i]) {
			continue
		}

		state := assignmentState{}
		state.scan(afterEquals(rootLines[i]))
		end := i + 1
		for state.needsContinuation() && end < len(rootLines) {
			state.scan(rootLines[end])
			end++
		}
		if state.needsContinuation() {
			return 0, 0, false, errors.New("unterminated top-level notify assignment")
		}
		return i, end, true, nil
	}
	return 0, 0, false, nil
}

func isNotifyAssignmentStart(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}

	keyEnd := 0
	for keyEnd < len(trimmed) {
		ch := trimmed[keyEnd]
		if ch == ' ' || ch == '\t' || ch == '=' {
			break
		}
		keyEnd++
	}
	if keyEnd == 0 || trimmed[:keyEnd] != "notify" {
		return false
	}

	rest := strings.TrimLeft(trimmed[keyEnd:], " \t")
	return strings.HasPrefix(rest, "=")
}

func afterEquals(line string) string {
	idx := strings.Index(line, "=")
	if idx == -1 {
		return ""
	}
	return line[idx+1:]
}

func renderNotifyLine(command []string) string {
	parts := make([]string, 0, len(command))
	for _, item := range command {
		parts = append(parts, quoteTOMLString(item))
	}
	return fmt.Sprintf("notify = [%s]", strings.Join(parts, ", "))
}

func quoteTOMLString(value string) string {
	replacer := strings.NewReplacer(
		"\\", `\\`,
		`"`, `\"`,
		"\t", `\t`,
		"\n", `\n`,
		"\r", `\r`,
	)
	return `"` + replacer.Replace(value) + `"`
}

func trimLeadingBlankLines(lines []string) []string {
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	return lines[i:]
}

type assignmentState struct {
	inString     bool
	escape       bool
	bracketDepth int
}

func (s *assignmentState) scan(line string) {
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if s.inString {
			if s.escape {
				s.escape = false
				continue
			}
			switch ch {
			case '\\':
				s.escape = true
			case '"':
				s.inString = false
			}
			continue
		}

		switch ch {
		case '"':
			s.inString = true
		case '#':
			return
		case '[':
			s.bracketDepth++
		case ']':
			if s.bracketDepth > 0 {
				s.bracketDepth--
			}
		}
	}
}

func (s assignmentState) needsContinuation() bool {
	return s.inString || s.bracketDepth > 0
}
