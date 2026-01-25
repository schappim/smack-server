package commands

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// InterpolationContext contains all the data available for variable substitution
type InterpolationContext struct {
	Input       string
	UserID      string
	Username    string
	DisplayName string
	ChannelID   string
	ChannelName string
}

// Interpolate replaces {{variable}} placeholders in a template string with actual values
func Interpolate(template string, ctx *InterpolationContext) string {
	return interpolate(template, ctx, false)
}

// InterpolateURL replaces {{variable}} placeholders and URL-encodes the values for safe use in URLs
func InterpolateURL(template string, ctx *InterpolationContext) string {
	return interpolate(template, ctx, true)
}

func interpolate(template string, ctx *InterpolationContext, urlEncode bool) string {
	result := template

	// Helper to optionally URL-encode values
	encode := func(s string) string {
		if urlEncode {
			return url.QueryEscape(s)
		}
		return s
	}

	// Parse input into words
	inputParts := strings.Fields(ctx.Input)

	// Replace {{input}} - full input text
	result = strings.ReplaceAll(result, "{{input}}", encode(ctx.Input))

	// Replace {{input.N}} where N is the word index (0-based)
	indexPattern := regexp.MustCompile(`\{\{input\.(\d+)\}\}`)
	result = indexPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := indexPattern.FindStringSubmatch(match)
		if len(matches) < 2 {
			return ""
		}
		idx, err := strconv.Atoi(matches[1])
		if err != nil || idx >= len(inputParts) {
			return ""
		}
		return encode(inputParts[idx])
	})

	// Replace {{input.rest}} - everything except first word
	if len(inputParts) > 1 {
		result = strings.ReplaceAll(result, "{{input.rest}}", encode(strings.Join(inputParts[1:], " ")))
	} else {
		result = strings.ReplaceAll(result, "{{input.rest}}", "")
	}

	// User variables
	result = strings.ReplaceAll(result, "{{user.id}}", encode(ctx.UserID))
	result = strings.ReplaceAll(result, "{{user.username}}", encode(ctx.Username))
	result = strings.ReplaceAll(result, "{{user.displayName}}", encode(ctx.DisplayName))

	// Channel variables
	result = strings.ReplaceAll(result, "{{channel.id}}", encode(ctx.ChannelID))
	result = strings.ReplaceAll(result, "{{channel.name}}", encode(ctx.ChannelName))

	// Time variables
	now := time.Now()
	result = strings.ReplaceAll(result, "{{timestamp}}", strconv.FormatInt(now.Unix(), 10))
	result = strings.ReplaceAll(result, "{{date}}", now.Format("2006-01-02"))
	result = strings.ReplaceAll(result, "{{datetime}}", now.Format(time.RFC3339))

	return result
}
