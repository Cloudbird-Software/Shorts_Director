package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cloudbird-Software/Shorts_Director/internal/contracts"
	"github.com/Cloudbird-Software/Shorts_Director/internal/videoplan"
)

const (
	shotRef  = "shot:018f6c01-aaaa-7aaa-8aaa-000000000002"
	shotHash = "sha256:1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a"
)

func loadPlan(t *testing.T) videoplan.Plan {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "schema", "testdata", "video_plan", "valid", "with_vo_and_speed.json"))
	if err != nil {
		t.Fatalf("读样本失败: %v", err)
	}
	var p videoplan.Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("解析 plan: %v", err)
	}
	return p
}

func fixtureIndex() MediaIndex {
	return MediaIndex{shotRef: MediaEntry{
		LocalPath: "/mnt/work/media/0002.mp4", ContentHash: shotHash, FPS: 25,
	}}
}

func fixtureFonts() []Font {
	return []Font{{Family: "HarmonyOS_Sans_Bold", Path: "/mnt/work/fonts/hsb.otf", Hash: "sha256:aaaa"}}
}

func fixtureOutput() Output {
	return Output{Path: "/mnt/work/out/final.mp4", Codec: "h264", CRF: 18, Preset: "slow"}
}

func fixtureModes() Modes      { return Modes{Deterministic: true} }
func fixtureExpect() RendererExpect {
	return RendererExpect{FFmpeg: "7.1", Remotion: "4.0.230", Node: "22.11.0"}
}

func compileOK(t *testing.T, idx MediaIndex, fonts []Font) *RenderRequest {
	t.Helper()
	req, err := Compile(loadPlan(t), idx, fonts, fixtureOutput(), fixtureModes(), fixtureExpect())
	if err != nil {
		t.Fatalf("Compile 意外失败: %v", err)
	}
	return req
}

// TestCompileSuccess：合法 plan + 完备索引/字体 → 契约形态完整。
func TestCompileSuccess(t *testing.T) {
	req := compileOK(t, fixtureIndex(), fixtureFonts())
	if req.ContractVersion != contracts.ContractRender {
		t.Errorf("contract_version = %d", req.ContractVersion)
	}
	if len(req.ResolvedMedia) != 1 || req.ResolvedMedia[0].Ref != shotRef {
		t.Fatalf("resolved_media = %+v", req.ResolvedMedia)
	}
	if req.ResolvedMedia[0].ContentHash != shotHash {
		t.Errorf("content_hash 未随解析结果带出: %s", req.ResolvedMedia[0].ContentHash)
	}
	if err := req.Validate(); err != nil {
		t.Errorf("产物未通过 Validate: %v", err)
	}
}

// TestCompileDeterministic：同输入恒同输出（排序后序列化逐字节相等）。
func TestCompileDeterministic(t *testing.T) {
	a, err := json.Marshal(compileOK(t, fixtureIndex(), fixtureFonts()))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(compileOK(t, fixtureIndex(), fixtureFonts()))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Error("两次编译输出不一致：确定性破坏")
	}
}

// TestCompileMissingMedia：索引缺条目 ⇒ R-4 报错，无隐式回退。
func TestCompileMissingMedia(t *testing.T) {
	_, err := Compile(loadPlan(t), MediaIndex{}, fixtureFonts(), fixtureOutput(), fixtureModes(), fixtureExpect())
	if err == nil || !strings.Contains(err.Error(), "R-4") {
		t.Errorf("缺媒体未触发 R-4: %v", err)
	}
}

// TestCompileIncompleteEntry：条目字段不全 ⇒ 报错。
func TestCompileIncompleteEntry(t *testing.T) {
	idx := MediaIndex{shotRef: {LocalPath: "/m/0002.mp4", ContentHash: shotHash}} // 缺 fps
	_, err := Compile(loadPlan(t), idx, fixtureFonts(), fixtureOutput(), fixtureModes(), fixtureExpect())
	if err == nil || !strings.Contains(err.Error(), "解析不完整") {
		t.Errorf("fps 缺失未触发: %v", err)
	}
}

// TestCompileHashDrift：素材重转码后 hash 漂移 ⇒ 旧 plan 显式失效。
func TestCompileHashDrift(t *testing.T) {
	idx := MediaIndex{shotRef: {
		LocalPath: "/m/0002.mp4", ContentHash: "sha256:ffff", FPS: 25,
	}}
	_, err := Compile(loadPlan(t), idx, fixtureFonts(), fixtureOutput(), fixtureModes(), fixtureExpect())
	if err == nil || !strings.Contains(err.Error(), "版本漂移") {
		t.Errorf("hash 漂移未触发: %v", err)
	}
}

// TestCompileFontMissing：overlay 引用未提供的字体 ⇒ R-4 报错。
func TestCompileFontMissing(t *testing.T) {
	fonts := []Font{{Family: "NotoSansSC", Path: "/f/noto.otf", Hash: "sha256:bbbb"}}
	_, err := Compile(loadPlan(t), fixtureIndex(), fonts, fixtureOutput(), fixtureModes(), fixtureExpect())
	if err == nil || !strings.Contains(err.Error(), "HarmonyOS_Sans_Bold") {
		t.Errorf("缺字体未触发: %v", err)
	}
}

// TestCompileConflictingHash：同一 ref 两个 content_hash ⇒ 报错。
func TestCompileConflictingHash(t *testing.T) {
	p := loadPlan(t)
	p.Tracks = append(p.Tracks, videoplan.Track{
		TrackID: "ins1", Kind: videoplan.TrackVideoInsert,
		Clips: []videoplan.Clip{{
			ClipID: "ins-c1", BeatRole: "PROOF",
			Source: videoplan.ClipSource{Kind: "SHOT", Ref: "018f6c01-aaaa-7aaa-8aaa-000000000002", ContentHash: "sha256:dead"},
			SrcIn: 0, SrcOut: 10, TlStart: 10, TlEnd: 20,
			Transform:    videoplan.Transform{Crop: &videoplan.Crop{W: 1080, H: 1920}, Scale: 1, Position: &videoplan.Position{}},
			TransitionIn: videoplan.TransitionIn{Kind: "CUT"},
		}},
	})
	_, err := Compile(p, fixtureIndex(), fixtureFonts(), fixtureOutput(), fixtureModes(), fixtureExpect())
	if err == nil || !strings.Contains(err.Error(), "两个 content_hash") {
		t.Errorf("hash 冲突未触发: %v", err)
	}
}

// TestCompileRejectsBadOutput：output.path/codec 必填。
func TestCompileRejectsBadOutput(t *testing.T) {
	empty := Output{}
	_, err := Compile(loadPlan(t), fixtureIndex(), fixtureFonts(), empty, fixtureModes(), fixtureExpect())
	if err == nil || !strings.Contains(err.Error(), "output") {
		t.Errorf("空 output 未触发: %v", err)
	}
}

// TestValidateMirror：Validate 是 C3 request schema 的 Go 侧镜像——
// codec 枚举 / renderer_expect 三元组 / ref 去重 / 媒体字段完备。
func TestValidateMirror(t *testing.T) {
	base := func() RenderRequest {
		r, _ := Compile(loadPlan(t), fixtureIndex(), fixtureFonts(), fixtureOutput(), fixtureModes(), fixtureExpect())
		return *r
	}

	r := base()
	r.ContractVersion = 2
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "contract_version") {
		t.Errorf("contract_version 镜像未触发: %v", err)
	}

	r = base()
	r.Output.Codec = "vp9"
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "codec") {
		t.Errorf("codec 枚举未触发: %v", err)
	}

	r = base()
	r.RendererExpect.Node = ""
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "renderer_expect") {
		t.Errorf("renderer_expect 未触发: %v", err)
	}

	r = base()
	r.ResolvedMedia = append(r.ResolvedMedia, r.ResolvedMedia[0])
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Errorf("ref 去重未触发: %v", err)
	}

	r = base()
	r.ResolvedMedia[0].ContentHash = ""
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "字段不全") {
		t.Errorf("字段完备未触发: %v", err)
	}

	r = base()
	r.Fonts[0].Hash = ""
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "fonts[0]") {
		t.Errorf("fonts 完备未触发: %v", err)
	}
}

// TestC3ValidSamplesValidate：C3 契约 valid 样本必须全部通过 Go 侧镜像校验
// （漂移由样本驱动发现——Go 实体与 JSON schema 双源一致）。
func TestC3ValidSamplesValidate(t *testing.T) {
	dir := filepath.Join("..", "..", "schema", "testdata", "contracts", "render", "request", "valid")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读目录失败: %v", err)
	}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("读 %s: %v", e.Name(), err)
		}
		var r RenderRequest
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Errorf("解析 %s: %v", e.Name(), err)
			continue
		}
		if err := r.Validate(); err != nil {
			t.Errorf("valid 样本 %s 未通过 Validate: %v", e.Name(), err)
		}
	}
}
