/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

For commercial licensing, please contact support@quantumnous.com
*/
package sora

import (
	"net/url"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// videoURLExtensions 视频直链常见后缀（小写，含前导点）。
var videoURLExtensions = []string{
	".mp4", ".mov", ".webm", ".mkv", ".m4v", ".m3u8", ".avi",
}

// extractVideoURLFromBody 递归遍历上游返回的整个 JSON，
// 挖出第一个「视频直链」URL（path 以视频后缀结尾的 http/https 链接）。
// 用于兼容各中转把视频链接嵌在不同深层字段里的情况；挖不到返回空串。
func extractVideoURLFromBody(respBody []byte) string {
	var root interface{}
	if err := common.Unmarshal(respBody, &root); err != nil {
		return ""
	}
	return findVideoURL(root)
}

// findVideoURL 深度优先遍历任意 JSON 节点，返回首个匹配的视频 URL。
// map 按 key 排序遍历以保证结果确定（同一 JSON 每次结果一致）。
func findVideoURL(node interface{}) string {
	switch v := node.(type) {
	case string:
		if isVideoURL(v) {
			return v
		}
	case []interface{}:
		for _, item := range v {
			if u := findVideoURL(item); u != "" {
				return u
			}
		}
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if u := findVideoURL(v[k]); u != "" {
				return u
			}
		}
	}
	return ""
}

// isVideoURL 判断字符串是否为「视频直链」：http/https 且 URL path 以视频后缀结尾。
// 用 URL.Path 判断，忽略后面的签名 query（如 OSS 的 ?OSSAccessKeyId=...&Signature=...）。
func isVideoURL(s string) bool {
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	path := strings.ToLower(u.Path)
	for _, ext := range videoURLExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}
