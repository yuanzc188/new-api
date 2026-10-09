package billingexpr

import (
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// imageSizePaths lists where each entrance carries the requested resolution:
// Gemini native generateContent (camel and snake), OpenAI chat converted to
// Gemini, and the OpenAI Images size. The first non-empty value wins.
var imageSizePaths = []string{
	"generationConfig.imageConfig.imageSize",
	"generationConfig.image_config.image_size",
	"generation_config.image_config.image_size",
	"extra_body.google.image_config.image_size",
	"size",
}

// Pixel-count boundaries between resolution tiers. 1536x1024 stays 1K,
// 2048x2048 and Gemini 2K 21:9 (3168x1344) stay 2K, 3840x2160 is 4K.
// ponytail: fixed thresholds; make them configurable if a provider's tiers stop fitting.
const (
	imageSize1KMaxPixels = 1536 * 1536
	imageSize2KMaxPixels = 2560 * 2560
	imageSizeMaxEdge     = 100_000
)

// ImageSizeTier normalizes the requested image resolution in a request body to
// "1K", "2K" or "4K". It returns "" when the request names no resolution or
// uses a value such as "auto", so expressions fall back to their default tier.
func ImageSizeTier(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	for _, path := range imageSizePaths {
		value := strings.TrimSpace(gjson.GetBytes(body, path).String())
		if value != "" {
			return normalizeImageSize(value)
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
	pixels := width * height
	if pixels <= imageSize1KMaxPixels {
		return "1K"
	}
	if pixels <= imageSize2KMaxPixels {
		return "2K"
	}
	return "4K"
}
