package sora

import "testing"

// 真实 dreamina 上游返回（视频链接嵌在 data.video_url / output[].url / result.* 等多处深层字段）。
const dreaminaBody = `{
  "id": "dreamina_2804",
  "code": 0,
  "data": {
    "job_id": 2804,
    "status": "success",
    "video_url": "https://dreamframe.oss-cn-chengdu.aliyuncs.com/uploads/results/20260707/0f3b1c0de10fac6d68fd8e06d74038b1.mp4?OSSAccessKeyId=LTAI5t8JJzDgv8EnAsxGsHBC&Expires=1783453077&Signature=abc%3D",
    "video_cover_url": ""
  },
  "output": [
    {
      "url": "https://dreamframe.oss-cn-chengdu.aliyuncs.com/uploads/results/20260707/0f3b1c0de10fac6d68fd8e06d74038b1.mp4?OSSAccessKeyId=LTAI5t8JJzDgv8EnAsxGsHBC&Expires=1783453077&Signature=abc%3D",
      "type": "video"
    }
  ],
  "status": "completed",
  "cover_url": null,
  "video_url": "https://dreamframe.oss-cn-chengdu.aliyuncs.com/uploads/results/20260707/0f3b1c0de10fac6d68fd8e06d74038b1.mp4?OSSAccessKeyId=LTAI5t8JJzDgv8EnAsxGsHBC&Expires=1783453077&Signature=abc%3D",
  "video_cover_url": ""
}`

const wantURL = "https://dreamframe.oss-cn-chengdu.aliyuncs.com/uploads/results/20260707/0f3b1c0de10fac6d68fd8e06d74038b1.mp4?OSSAccessKeyId=LTAI5t8JJzDgv8EnAsxGsHBC&Expires=1783453077&Signature=abc%3D"

func TestExtractVideoURLFromBody_NestedMp4(t *testing.T) {
	got := extractVideoURLFromBody([]byte(dreaminaBody))
	if got != wantURL {
		t.Fatalf("提取视频 URL 失败\n got: %q\nwant: %q", got, wantURL)
	}
}

func TestExtractVideoURLFromBody_NoVideoReturnsEmpty(t *testing.T) {
	// 真 OpenAI sora 的 completed 响应不含视频直链，应返回空以回退代理端点。
	body := `{"id":"video_68d...","status":"completed","progress":100,"seconds":"8"}`
	if got := extractVideoURLFromBody([]byte(body)); got != "" {
		t.Fatalf("无视频链接时应返回空，got: %q", got)
	}
}

func TestExtractVideoURLFromBody_IgnoresCoverImage(t *testing.T) {
	// 封面是图片后缀，不应被当成视频；只应抓 .mp4。
	body := `{"cover_url":"https://x.com/a/cover.jpg","output":[{"url":"https://x.com/v/clip.mp4?sig=1"}]}`
	want := "https://x.com/v/clip.mp4?sig=1"
	if got := extractVideoURLFromBody([]byte(body)); got != want {
		t.Fatalf("应跳过封面图只抓视频\n got: %q\nwant: %q", got, want)
	}
}
