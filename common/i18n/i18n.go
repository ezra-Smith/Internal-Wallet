package i18n

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"internalwallet/common/middleware"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

var (
	supportedTags = []language.Tag{
		language.MustParse("zh-CN"),
		language.MustParse("en-US"),
	}
	matcher    = language.NewMatcher(supportedTags)
	defaultTag = language.MustParse("zh-CN")

	loadOnce sync.Once

	templatesByLocale = map[string]map[string]*template.Template{} // locale -> key -> template
)

// Locale returns the best-matched locale for this request (e.g., "zh-CN" or "en-US").
func Locale(ctx context.Context) string {
	loadOnce.Do(loadTranslations)

	raw := middleware.GetLocale(ctx)
	tag := matchLocale(raw)
	return tag.String()
}

// T returns the localized message for key, rendered with optional template data.
//
// Translation lookup:
// 1) exact match for request locale
// 2) fallback to default locale
// 3) fallback to key itself
func T(ctx context.Context, key string, data map[string]interface{}) string {
	loadOnce.Do(loadTranslations)

	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}

	locale := Locale(ctx)
	if msg := render(locale, key, data); msg != "" {
		return msg
	}
	if locale != defaultTag.String() {
		if msg := render(defaultTag.String(), key, data); msg != "" {
			return msg
		}
	}
	return key
}

func matchLocale(raw string) language.Tag {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultTag
	}
	raw = strings.ReplaceAll(raw, "_", "-")

	// Prefer RFC 2616 Accept-Language parsing (also works for simple tags like "zh-CN").
	if tags, _, err := language.ParseAcceptLanguage(raw); err == nil && len(tags) > 0 {
		tag, _, _ := matcher.Match(tags...)
		return tag
	}

	// Fallback: try parsing as a single BCP 47 tag.
	if tag, err := language.Parse(raw); err == nil {
		matched, _, _ := matcher.Match(tag)
		return matched
	}
	return defaultTag
}

func loadTranslations() {
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		logx.Errorf("i18n: failed to read embedded locales: %v", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		locale := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		b, err := localeFS.ReadFile(filepath.Join("locales", entry.Name()))
		if err != nil {
			logx.Errorf("i18n: read %s failed: %v", entry.Name(), err)
			continue
		}

		var raw map[string]string
		if err := json.Unmarshal(b, &raw); err != nil {
			logx.Errorf("i18n: parse %s failed: %v", entry.Name(), err)
			continue
		}

		m := make(map[string]*template.Template, len(raw))
		for key, val := range raw {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			tpl, err := template.New(key).Option("missingkey=zero").Parse(val)
			if err != nil {
				logx.Errorf("i18n: template parse failed (locale=%s key=%s): %v", locale, key, err)
				continue
			}
			m[key] = tpl
		}

		templatesByLocale[locale] = m
	}
}

func render(locale string, key string, data map[string]interface{}) string {
	m := templatesByLocale[locale]
	if len(m) == 0 {
		return ""
	}
	tpl := m[key]
	if tpl == nil {
		return ""
	}

	// Fast-path: no template data.
	if len(data) == 0 {
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, map[string]interface{}{}); err != nil {
			return ""
		}
		return strings.TrimSpace(buf.String())
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}
