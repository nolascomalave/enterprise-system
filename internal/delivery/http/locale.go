package http

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type localeContextKey struct{}

const defaultLocale = "es"

// LocaleFromContext returns the locale resolved by LocaleMiddleware.
func LocaleFromContext(ctx context.Context) string {
	if ctx == nil {
		return defaultLocale
	}
	if locale, ok := ctx.Value(localeContextKey{}).(string); ok && locale != "" {
		return locale
	}
	return defaultLocale
}

// LocaleMiddleware resolves Accept-Language into a supported BCP 47 locale and
// stores it in the request context. Unsupported or missing headers fall back to
// Spanish.
func LocaleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		locale := resolveLocale(c.GetHeader("Accept-Language"))
		ctx := context.WithValue(c.Request.Context(), localeContextKey{}, locale)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func resolveLocale(header string) string {
	best := ""
	bestQ := -1.0
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		tag := strings.TrimSpace(fields[0])
		if tag == "" {
			continue
		}
		q := 1.0
		for _, param := range fields[1:] {
			param = strings.TrimSpace(param)
			if strings.HasPrefix(param, "q=") {
				if value, err := strconv.ParseFloat(strings.TrimPrefix(param, "q="), 64); err == nil {
					q = value
				}
			}
		}
		if q <= 0 {
			continue
		}
		if locale := supportedLocale(tag); locale != "" && q > bestQ {
			best = locale
			bestQ = q
		}
	}
	if best == "" {
		return defaultLocale
	}
	return best
}

func supportedLocale(tag string) string {
	lower := strings.ToLower(strings.TrimSpace(tag))
	switch {
	case lower == "pt" || lower == "pt-br" || strings.HasPrefix(lower, "pt-"):
		return "pt-BR"
	case lower == "es" || strings.HasPrefix(lower, "es-"):
		return "es"
	case lower == "en" || strings.HasPrefix(lower, "en-"):
		return "en"
	default:
		return ""
	}
}
