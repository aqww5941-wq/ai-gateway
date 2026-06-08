package filter

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Rule defines a named PII detection pattern.
type Rule struct {
	Name        string
	Label       string
	Pattern     *regexp.Regexp
	Description string
}

// Mode controls what happens when PII is detected.
type Mode string

const (
	ModeMask  Mode = "mask"  // Replace PII with [REDACTED_<label>]
	ModeBlock Mode = "block" // Reject the request entirely
)

// Filter detects and handles sensitive information in text.
type Filter struct {
	rules []Rule
	mode  Mode
}

var defaultRules = []struct {
	Name        string
	Label       string
	Regex       string
	Description string
}{
	{"phone_cn", "手机号", `1[3-9]\d{9}`, "Chinese mobile phone number"},
	{"id_card_cn", "身份证号", `\d{17}[\dXx]`, "Chinese 18-digit ID card number"},
	{"email", "邮箱", `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`, "Email address"},
	{"credit_card", "银行卡号", `\d{4}[\s\-]?\d{4}[\s\-]?\d{4}[\s\-]?\d{4}`, "Credit / debit card number"},
	{"ipv4", "IPv4地址", `\b(?:\d{1,3}\.){3}\d{1,3}\b`, "IPv4 address"},
	{"api_key", "API密钥", `sk-[a-f0-9]{16,128}`, "API key in sk-<hex> format"},
	{"cn_name", "中文姓名", `(?:王|李|张|刘|陈|杨|黄|赵|周|吴|徐|孙|马|胡|朱|郭|何|罗|高|林)[\p{Han}]{1,2}`, "Common Chinese surnames + given name (may over-match)"},
}

var rulesByName = make(map[string]struct {
	Label string
	Regex string
	Desc  string
})

func init() {
	for _, r := range defaultRules {
		rulesByName[r.Name] = struct {
			Label string
			Regex string
			Desc  string
		}{r.Label, r.Regex, r.Description}
	}
}

// AvailableRules returns the names of all built-in rules.
func AvailableRules() []map[string]string {
	out := make([]map[string]string, 0, len(defaultRules))
	for _, r := range defaultRules {
		out = append(out, map[string]string{
			"name":        r.Name,
			"label":       r.Label,
			"description": r.Description,
		})
	}
	return out
}

// New creates a Filter with the given enabled rule names and mode.
// An empty enabled list disables filtering entirely.
func New(enabled []string, mode Mode) *Filter {
	if len(enabled) == 0 || mode == "" {
		return nil
	}
	if mode != ModeMask && mode != ModeBlock {
		mode = ModeMask
	}

	var rules []Rule
	for _, name := range enabled {
		info, ok := rulesByName[name]
		if !ok {
			continue
		}
		re, err := regexp.Compile(info.Regex)
		if err != nil {
			continue
		}
		rules = append(rules, Rule{
			Name:        name,
			Label:       info.Label,
			Pattern:     re,
			Description: info.Desc,
		})
	}
	if len(rules) == 0 {
		return nil
	}
	// Sort by pattern length descending so longer patterns (e.g. ID card)
	// match before shorter ones (e.g. phone) that may be substrings.
	sort.Slice(rules, func(i, j int) bool {
		return len(rules[i].Pattern.String()) > len(rules[j].Pattern.String())
	})
	return &Filter{rules: rules, mode: mode}
}

// Scan checks text for PII and returns the list of detected rule labels.
// Returns nil if nothing was detected.
func (f *Filter) Scan(text string) []string {
	if f == nil {
		return nil
	}
	var found []string
	for _, r := range f.rules {
		if r.Pattern.MatchString(text) {
			found = append(found, r.Label)
		}
	}
	return found
}

// Apply filters sensitive information from text.
// In mask mode, replaces matches with [REDACTED_<label>].
// In block mode, returns an error if any PII is detected.
//
// The second return value is the list of rule labels that were triggered.
func (f *Filter) Apply(text string) (string, []string, error) {
	if f == nil || text == "" {
		return text, nil, nil
	}

	var triggered []string
	result := text

	for _, r := range f.rules {
		matches := r.Pattern.FindAllString(result, -1)
		if len(matches) > 0 {
			triggered = append(triggered, r.Label)
			if f.mode == ModeMask {
				replacement := "[REDACTED_" + r.Name + "]"
				// Use a single replacement per unique match to avoid re-matching
				// the replacement text. Replace each occurrence one at a time.
				for _, match := range matches {
					result = strings.Replace(result, match, replacement, 1)
				}
			}
		}
	}

	if f.mode == ModeBlock && len(triggered) > 0 {
		return text, triggered, fmt.Errorf("sensitive information detected: %s", strings.Join(triggered, ", "))
	}

	return result, triggered, nil
}

// FilterMessages applies PII filtering to an array of message contents,
// returning the filtered messages and any triggered rules.
func (f *Filter) FilterMessages(messages []map[string]string) ([]map[string]string, []string, error) {
	if f == nil {
		return messages, nil, nil
	}

	var allTriggered []string
	filtered := make([]map[string]string, len(messages))

	for i, msg := range messages {
		filtered[i] = make(map[string]string, len(msg))
		for k, v := range msg {
			if k == "role" || k == "name" {
				filtered[i][k] = v
				continue
			}
			cleaned, triggered, err := f.Apply(v)
			if err != nil {
				return messages, triggered, err
			}
			filtered[i][k] = cleaned
			allTriggered = append(allTriggered, triggered...)
		}
	}

	return filtered, allTriggered, nil
}
