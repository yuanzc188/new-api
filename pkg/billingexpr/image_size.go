package billingexpr

import (
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// imageSizePaths lists where each entrance carries the requested resolution:
// Gemini native generateContent (camel and snake), OpenAI chat converted to
// Gemini, the third-party image_size field and the OpenAI Images size. The
// first value that names a tier wins, so an aspect-ratio size such as "16:9"
// does not hide an image_size of "4K".
var imageSizePaths = []string{
	"generationConfig.imageConfig.imageSize",
	"generationConfig.image_config.image_size",
	"generation_config.image_config.image_size",
	"extra_body.google.image_config.image_size",
	"image_size",
	"size",
}

// A WxH size reaches a tier when either its long edge or its pixel count
// exceeds the lower tier. The long edge catches wide ratios whose pixel count
// stays low: gpt-image 21:9 at 3840x1648 is only ~6.3MP but is the 4K option,
// while Gemini 2K 21:9 (3168x1344) stays 2K.
// ponytail: fixed thresholds; make them configurable if a provider's tiers stop fitting.
const (
	imageSize1KMaxLongEdge = 1600
	imageSize2KMaxLongEdge = 3200
	imageSize1KMaxPixels   = 1536 * 1536
	imageSize2KMaxPixels   = 2560 * 2560
	imageSizeMaxEdge       = 100_000
)

// ImageSizeTier normalizes the requested image resolution in a request body to
// "1K", "2K" or "4K". It returns "" when the request names no resolution or
// uses a value such as "auto", so expressions fall back to their default tier.
func ImageSizeTier(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	for _, path := range imageSizePaths {
		if tier := normalizeImageSize(strings.TrimSpace(gjson.GetBytes(body, path).String())); tier != "" {
			return tier
		}
	}
	return ""
}

func normalizeImageSize(value string) string {
	value = strings.ToUpper(value)
	switch value {
	case "1K", "2K", "4K":
		return value
	}
	widthText, heightText, ok := strings.Cut(value, "X")
	if !ok {
		return ""
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(widthText))
	height, heightErr := strconv.Atoi(strings.TrimSpace(heightText))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 || width > imageSizeMaxEdge || height > imageSizeMaxEdge {
		return ""
	}
	longEdge, pixels := max(width, height), width*height
	if longEdge <= imageSize1KMaxLongEdge && pixels <= imageSize1KMaxPixels {
		return "1K"
	}
	if longEdge <= imageSize2KMaxLongEdge && pixels <= imageSize2KMaxPixels {
		return "2K"
	}
	return "4K"
}
