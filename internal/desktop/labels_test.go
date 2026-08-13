package desktop

import "testing"

func TestNormalizeLocale(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "zh-CN"},
		{"zh-CN", "zh-CN"},
		{"zh", "zh-CN"},
		{"en-US", "en-US"},
		{"en", "en-US"},
		{"en-GB", "en-US"},
		{"fr-FR", "zh-CN"},
		{"  en-US  ", "en-US"},
	}
	for _, tt := range tests {
		if got := NormalizeLocale(tt.in); got != tt.want {
			t.Errorf("NormalizeLocale(%q) = %q want %q", tt.in, got, tt.want)
		}
	}
}

func TestLabelsForLocale(t *testing.T) {
	zh := LabelsForLocale("zh-CN")
	wantZH := Labels{
		OpenWindow:       "打开窗口",
		RefreshFeeds:     "刷新订阅",
		EnableWebAccess:  "开启 Web 访问",
		DisableWebAccess: "关闭 Web 访问",
		Quit:             "退出",
	}
	if zh != wantZH {
		t.Fatalf("zh-CN labels = %+v want %+v", zh, wantZH)
	}

	en := LabelsForLocale("en-US")
	wantEN := Labels{
		OpenWindow:       "Open window",
		RefreshFeeds:     "Refresh feeds",
		EnableWebAccess:  "Enable web access",
		DisableWebAccess: "Disable web access",
		Quit:             "Quit",
	}
	if en != wantEN {
		t.Fatalf("en-US labels = %+v want %+v", en, wantEN)
	}

	if got := LabelsForLocale(""); got != wantZH {
		t.Fatalf("default locale labels = %+v want zh-CN", got)
	}
	if got := LabelsForLocale("en"); got != wantEN {
		t.Fatalf("en alias labels = %+v want en-US", got)
	}
}

func TestWebAccessActionLabel(t *testing.T) {
	zh := LabelsForLocale("zh-CN")
	if got := WebAccessActionLabel(zh, true); got != "关闭 Web 访问" {
		t.Fatalf("on = %q", got)
	}
	if got := WebAccessActionLabel(zh, false); got != "开启 Web 访问" {
		t.Fatalf("off = %q", got)
	}
	en := LabelsForLocale("en-US")
	if got := WebAccessActionLabel(en, true); got != "Disable web access" {
		t.Fatalf("en on = %q", got)
	}
	if got := WebAccessActionLabel(en, false); got != "Enable web access" {
		t.Fatalf("en off = %q", got)
	}
}
