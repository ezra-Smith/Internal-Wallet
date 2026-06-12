package logic

import "testing"

func TestMaskNicknameIfDefault_Phone(t *testing.T) {
	out := maskNicknameIfDefault("13199998888", "13199998888", "86", "u@example.com")
	if out != "131****8888" {
		t.Fatalf("expected %q, got %q", "131****8888", out)
	}
}

func TestMaskNicknameIfDefault_Phone_WithCountryCode(t *testing.T) {
	out1 := maskNicknameIfDefault("+8613199998888", "13199998888", "86", "")
	if out1 != "131****8888" {
		t.Fatalf("expected %q, got %q", "131****8888", out1)
	}

	out2 := maskNicknameIfDefault("8613199998888", "13199998888", "86", "")
	if out2 != "131****8888" {
		t.Fatalf("expected %q, got %q", "131****8888", out2)
	}
}

func TestMaskNicknameIfDefault_Email(t *testing.T) {
	out := maskNicknameIfDefault("AbCd@Example.com", "", "", "abcd@example.com")
	if out != "a**d@example.com" {
		t.Fatalf("expected %q, got %q", "a**d@example.com", out)
	}
}

func TestMaskNicknameIfDefault_Custom(t *testing.T) {
	out := maskNicknameIfDefault("自定义昵称", "13199998888", "86", "u@example.com")
	if out != "自定义昵称" {
		t.Fatalf("expected %q, got %q", "自定义昵称", out)
	}
}
