package filter

import (
	"strings"
	"testing"
)

func TestMaskPhone(t *testing.T) {
	f := New([]string{"phone_cn"}, ModeMask)
	if f == nil {
		t.Fatal("expected filter, got nil")
	}

	out, triggered, err := f.Apply("你好，我的电话是13812345678，请回电。")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triggered) == 0 {
		t.Fatal("expected phone to be detected")
	}
	if strings.Contains(out, "13812345678") {
		t.Errorf("phone number was not masked: %s", out)
	}
	if !strings.Contains(out, "[REDACTED_phone_cn]") {
		t.Errorf("expected REDACTED marker: %s", out)
	}
}

func TestMaskIDCard(t *testing.T) {
	f := New([]string{"id_card_cn"}, ModeMask)
	out, triggered, err := f.Apply("身份证号：110101199001011234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triggered) == 0 {
		t.Fatal("expected ID card to be detected")
	}
	if strings.Contains(out, "110101199001011234") {
		t.Errorf("ID card was not masked: %s", out)
	}
}

func TestMaskEmail(t *testing.T) {
	f := New([]string{"email"}, ModeMask)
	out, triggered, err := f.Apply("邮箱是 test@example.com 和 admin@corp.cn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triggered) == 0 {
		t.Fatal("expected email to be detected")
	}
	if strings.Contains(out, "test@example.com") {
		t.Errorf("email was not masked")
	}
}

func TestBlockMode(t *testing.T) {
	f := New([]string{"phone_cn"}, ModeBlock)
	_, triggered, err := f.Apply("电话 13800001111")
	if err == nil {
		t.Fatal("expected block error")
	}
	if len(triggered) == 0 {
		t.Fatal("expected phone to be triggered")
	}
	if !strings.Contains(err.Error(), "手机号") {
		t.Errorf("error should mention 手机号: %v", err)
	}
}

func TestNoMatch(t *testing.T) {
	f := New([]string{"phone_cn", "email"}, ModeMask)
	out, triggered, err := f.Apply("普通文本，没有敏感信息。")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triggered) != 0 {
		t.Errorf("expected no triggers, got: %v", triggered)
	}
	if out != "普通文本，没有敏感信息。" {
		t.Errorf("text should be unchanged: %s", out)
	}
}

func TestNilFilter(t *testing.T) {
	var f *Filter
	out, triggered, err := f.Apply("电话 13800001111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triggered) != 0 {
		t.Errorf("nil filter should not trigger")
	}
	if out != "电话 13800001111" {
		t.Errorf("nil filter should pass through")
	}
}

func TestEmptyEnabled(t *testing.T) {
	f := New(nil, ModeMask)
	if f != nil {
		t.Fatal("expected nil filter for empty rules")
	}
	f = New([]string{}, ModeMask)
	if f != nil {
		t.Fatal("expected nil filter for empty rules")
	}
}

func TestAvailableRules(t *testing.T) {
	rules := AvailableRules()
	if len(rules) == 0 {
		t.Fatal("expected some available rules")
	}
}

func TestFilterMessages(t *testing.T) {
	f := New([]string{"phone_cn"}, ModeMask)
	msgs := []map[string]string{
		{"role": "user", "content": "电话13800001111"},
		{"role": "assistant", "content": "好的，13800001111记下了"},
	}
	filtered, triggered, err := f.FilterMessages(msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triggered) < 2 {
		t.Errorf("expected 2 triggers, got %d", len(triggered))
	}
	for _, msg := range filtered {
		if strings.Contains(msg["content"], "13800001111") {
			t.Errorf("phone not masked in message: %v", msg)
		}
	}
}

func TestIDCardBeforePhone(t *testing.T) {
	// The ID card rule (18 chars) must match before phone (11 chars)
	// to prevent phone regex from breaking the ID card substring.
	f := New([]string{"phone_cn", "id_card_cn"}, ModeMask)
	out, triggered, err := f.Apply("身份证110101199001011234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "[REDACTED_id_card_cn]") {
		// ID card matched as a whole — correct
	} else if strings.Contains(out, "[REDACTED_phone_cn]") {
		t.Errorf("phone matched inside ID card, should have been id_card first: %s", out)
	}
	if len(triggered) == 0 {
		t.Error("expected at least one trigger")
	}
	// The full 18-digit should not appear in output
	if strings.Contains(out, "110101199001011234") {
		t.Error("ID card not masked")
	}
}

func TestFilterMessagesBlock(t *testing.T) {
	f := New([]string{"phone_cn"}, ModeBlock)
	msgs := []map[string]string{
		{"role": "user", "content": "电话13800001111"},
	}
	_, triggered, err := f.FilterMessages(msgs)
	if err == nil {
		t.Fatal("expected block error")
	}
	if len(triggered) == 0 {
		t.Fatal("expected phone to be triggered")
	}
}
