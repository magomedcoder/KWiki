package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

const (
	DefaultLang = "ru"
	LangCookie  = "kwiki_lang"
)

type Bundle struct {
	defaultLang string
	messages    map[string]map[string]string
	order       []string
}

type Language struct {
	Code  string
	Label string
}

func Load(fsys fs.FS) (*Bundle, error) {
	return LoadWithDefault(fsys, DefaultLang)
}

func LoadWithDefault(fsys fs.FS, defaultLang string) (*Bundle, error) {
	defaultLang = Normalize(defaultLang)
	if defaultLang == "" {
		defaultLang = DefaultLang
	}

	entries, err := fs.ReadDir(fsys, "locales")
	if err != nil {
		return nil, err
	}

	messages := make(map[string]map[string]string)
	var order []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		code := Normalize(strings.TrimSuffix(entry.Name(), ".json"))
		if code == "" {
			continue
		}

		raw, err := fs.ReadFile(fsys, path.Join("locales", entry.Name()))
		if err != nil {
			return nil, err
		}

		var catalog map[string]string
		if err := json.Unmarshal(raw, &catalog); err != nil {
			return nil, fmt.Errorf("locale %s: %w", code, err)
		}

		messages[code] = catalog
		order = append(order, code)
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no locales found")
	}

	sort.Strings(order)
	if _, ok := messages[defaultLang]; !ok {
		defaultLang = order[0]
	}

	return &Bundle{
		defaultLang: defaultLang,
		messages:    messages,
		order:       order,
	}, nil
}

func (b *Bundle) Default() string {
	if b == nil || b.defaultLang == "" {
		return DefaultLang
	}

	return b.defaultLang
}

func (b *Bundle) Supported(lang string) bool {
	if b == nil {
		return false
	}

	_, ok := b.messages[Normalize(lang)]
	return ok
}

func (b *Bundle) Languages() []Language {
	if b == nil {
		return nil
	}

	out := make([]Language, 0, len(b.order))
	for _, code := range b.order {
		label := strings.ToUpper(code)
		out = append(out, Language{
			Code:  code,
			Label: label,
		})
	}

	return out
}

func (b *Bundle) Locale(lang string) string {
	lang = b.resolve(lang)
	if loc := b.lookup(lang, "meta.locale"); loc != "" {
		return loc
	}

	switch lang {
	case "en":
		return "en_US"
	default:
		return "ru_RU"
	}
}

func (b *Bundle) T(lang, key string, args ...any) string {
	if key == "" {
		return ""
	}

	lang = b.resolve(lang)
	msg := b.lookup(lang, key)
	if msg == "" && lang != b.Default() {
		msg = b.lookup(b.Default(), key)
	}

	if msg == "" {
		msg = key
	}

	if len(args) == 0 {
		return msg
	}

	return fmt.Sprintf(msg, args...)
}

func (b *Bundle) ClientMessages(lang string) map[string]string {
	lang = b.resolve(lang)
	out := map[string]string{}
	for _, catalog := range []map[string]string{b.messages[b.Default()], b.messages[lang]} {
		for key, value := range catalog {
			if strings.HasPrefix(key, "js.") {
				out[key] = value
			}
		}
	}

	return out
}

func (b *Bundle) Resolve(cookie, accept string) string {
	if b.Supported(cookie) {
		return Normalize(cookie)
	}

	if lang := ParseAcceptLanguage(accept, b.order); lang != "" {
		return lang
	}

	return b.Default()
}

func (b *Bundle) resolve(lang string) string {
	lang = Normalize(lang)
	if b != nil && b.Supported(lang) {
		return lang
	}

	return b.Default()
}

func (b *Bundle) lookup(lang, key string) string {
	if b == nil {
		return ""
	}

	if catalog := b.messages[lang]; catalog != nil {
		return catalog[key]
	}

	return ""
}

func Normalize(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if lang == "" {
		return ""
	}

	if i := strings.IndexAny(lang, "-_"); i >= 0 {
		lang = lang[:i]
	}

	for _, r := range lang {
		if r < 'a' || r > 'z' {
			return ""
		}
	}

	return lang
}

func ParseAcceptLanguage(header string, supported []string) string {
	if header == "" || len(supported) == 0 {
		return ""
	}

	type offer struct {
		tag string
		q   float64
	}

	var offers []offer
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		tagPart, rest, _ := strings.Cut(part, ";")
		tag := strings.TrimSpace(tagPart)
		norm := Normalize(tag)
		if tag != "*" && norm == "" {
			continue
		}

		if tag == "*" {
			norm = "*"
		}

		q := 1.0
		for param := range strings.SplitSeq(rest, ";") {
			param = strings.TrimSpace(param)
			if after, ok := strings.CutPrefix(param, "q="); ok {
				if v, err := strconv.ParseFloat(strings.TrimSpace(after), 64); err == nil {
					q = v
				}
			}
		}

		offers = append(offers, offer{tag: norm, q: q})
	}

	sort.SliceStable(offers, func(i, j int) bool {
		return offers[i].q > offers[j].q
	})

	supportedSet := make(map[string]bool, len(supported))
	for _, lang := range supported {
		supportedSet[Normalize(lang)] = true
	}

	for _, item := range offers {
		if item.tag == "*" {
			return Normalize(supported[0])
		}

		if supportedSet[item.tag] {
			return item.tag
		}
	}

	return ""
}
