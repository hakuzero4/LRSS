package desktop

import "strings"

// Labels are tray context-menu strings for one UI locale.
type Labels struct {
	OpenWindow       string
	RefreshFeeds     string
	EnableWebAccess  string
	DisableWebAccess string
	Quit             string
}

// NormalizeLocale maps UIPrefs.locale (and common aliases) to "zh-CN" or "en-US".
// Empty or unknown values default to zh-CN.
func NormalizeLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return "zh-CN"
	}
	switch locale {
	case "en-US", "en", "en-us", "en_US", "en-GB", "en-gb":
		return "en-US"
	case "zh-CN", "zh", "zh-cn", "zh_CN", "zh-Hans":
		return "zh-CN"
	default:
		low := strings.ToLower(locale)
		if strings.HasPrefix(low, "en") {
			return "en-US"
		}
		return "zh-CN"
	}
}

// LabelsForLocale returns tray menu copy for locale. Default is zh-CN.
func LabelsForLocale(locale string) Labels {
	if NormalizeLocale(locale) == "en-US" {
		return Labels{
			OpenWindow:       "Open window",
			RefreshFeeds:     "Refresh feeds",
			EnableWebAccess:  "Enable web access",
			DisableWebAccess: "Disable web access",
			Quit:             "Quit",
		}
	}
	return Labels{
		OpenWindow:       "打开窗口",
		RefreshFeeds:     "刷新订阅",
		EnableWebAccess:  "开启 Web 访问",
		DisableWebAccess: "关闭 Web 访问",
		Quit:             "退出",
	}
}

// WebAccessActionLabel is the toggle caption: disable when on, enable when off.
func WebAccessActionLabel(labels Labels, webAccessEnabled bool) string {
	if webAccessEnabled {
		return labels.DisableWebAccess
	}
	return labels.EnableWebAccess
}
