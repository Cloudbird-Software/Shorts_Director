package contracts

import "testing"

// TestSchemaVersionAnchors 守住 Go 侧锚点与 schema/entities 文件一一对应。
func TestSchemaVersionAnchors(t *testing.T) {
	got := map[SchemaName]string{}
	for _, n := range []SchemaName{
		SchemaBrandKernel, SchemaShot, SchemaAsset, SchemaShotSlotQuery,
		SchemaVideoPlan, SchemaQCAssertion, SchemaProductionOrder, SchemaEvent,
	} {
		got[n] = SchemaVersion(n)
	}
	want := map[SchemaName]string{
		SchemaBrandKernel:     "brand_kernel/1",
		SchemaShot:            "shot/1",
		SchemaAsset:           "asset/1",
		SchemaShotSlotQuery:   "shot_slot_query/1",
		SchemaVideoPlan:       "video_plan/1",
		SchemaQCAssertion:     "qc_assertion/1",
		SchemaProductionOrder: "production_order/1",
		SchemaEvent:           "event/1",
	}
	for n, w := range want {
		if got[n] != w {
			t.Errorf("SchemaVersion(%s) = %q, want %q", n, got[n], w)
		}
	}
}

func TestDefaultCanvas(t *testing.T) {
	c := DefaultCanvas()
	if c.Width != 1080 || c.Height != 1920 || len(c.FPS) != 2 || c.FPS[0] != 25 || c.FPS[1] != 30 {
		t.Fatalf("DefaultCanvas() = %+v", c)
	}
}

func TestDefaultRenderCraft(t *testing.T) {
	r := DefaultRenderCraft()
	if r.MaxSpeed != 1.15 || r.BeatSnapToleranceFrames != 3 || r.TargetLufs != -14 {
		t.Fatalf("DefaultRenderCraft() = %+v", r)
	}
}
