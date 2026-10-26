package cost

import "strings"

// ParseTagGroupKey extracts the App tag value from a Cost Explorer group key (e.g. "Application$APP-CHK", "Application$").
func ParseTagGroupKey(key string) string {
	if i := strings.LastIndex(key, "$"); i >= 0 {
		return strings.TrimSpace(key[i+1:])
	}
	return strings.TrimSpace(key)
}

// TagMatchesApp reports whether tag value matches app code case-insensitively (D-011).
func TagMatchesApp(tag, appCode string) bool {
	return strings.EqualFold(strings.TrimSpace(tag), strings.TrimSpace(appCode))
}
