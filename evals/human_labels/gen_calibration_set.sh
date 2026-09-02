#!/bin/sh
# 裁判校准集生成器（IR-0007 AC-8 / L-04 adversarial_corpus）：
# 确定性合成 100 条媒体（4 类 × 25）+ 首过人工标注 labels.json。
#   sharp_XXX   清晰渐变（可用 = true）
#   blur_XXX    模糊（L-04 对抗：不可用）
#   black_XXX   全黑/近全黑（L-04 对抗：不可用）
#   distort_XXX 像素化畸变（L-04 对抗：不可用）
# ffmpeg lavfi 零依赖、同参数恒同输出（与 evals/gen_seed_images.sh 同法）。
# 标注为开发者首过人工判定（合成媒体、无真实人物——INV-6）；
# V100 真实校准前可替换为多人标注（labeler 字段留痕）。
set -eu
cd "$(dirname "$0")"
mkdir -p media

# 5 组调色板按序循环，保证同类别内条目互异且确定
set -- "0x8B1A1A:0x1A0E08" "0x3E2B1F:0x0E1418" "0x6C3B7A:0x140E18" \
  "0x2A3B6C:0x101418" "0x1F5A3E:0x0E1810"

idx=1
while [ "$idx" -le 25 ]; do
  c=$(eval "echo \${$(( (idx - 1) % 5 + 1 ))}")
  c0=${c%%:*}
  c1=${c##*:}
  n=$(printf '%03d' "$idx")
  ffmpeg -y -loglevel error -f lavfi \
    -i "gradients=s=256x256:c0=$c0:c1=$c1:speed=0.001" -frames:v 1 "media/sharp_$n.png"
  ffmpeg -y -loglevel error -f lavfi \
    -i "gradients=s=256x256:c0=$c0:c1=$c1:speed=0.001" \
    -vf "boxblur=10:1" -frames:v 1 "media/blur_$n.png"
  ffmpeg -y -loglevel error -f lavfi \
    -i "color=c=$(printf '0x%02x0303' $((idx % 4))):s=256x256" -frames:v 1 "media/black_$n.png"
  ffmpeg -y -loglevel error -f lavfi \
    -i "gradients=s=256x256:c0=$c0:c1=$c1:speed=0.001" \
    -vf "scale=16:16,scale=256:256:flags=neighbor" -frames:v 1 "media/distort_$n.png"
  idx=$((idx + 1))
done

# labels.json：单题可用性口径（与 README E6 表述一致）
python3 - <<'PY'
import json

QUESTION = "该画面是否可直接用于营销视频成片（清晰、非全黑、无明显畸变）？"
CATS = [
    ("sharp", True, "清晰渐变，首过人工判定可用"),
    ("blur", False, "L-04 对抗：模糊不可用"),
    ("black", False, "L-04 对抗：全黑不可用"),
    ("distort", False, "L-04 对抗：像素化畸变不可用"),
]
items = []
for cat, label, note in CATS:
    for i in range(1, 26):
        items.append({
            "item_id": "%s_%03d" % (cat, i),
            "media_path": "media/%s_%03d.png" % (cat, i),
            "question": QUESTION,
            "human_label": label,
            "labeler": "human-first-pass@1",
            "notes": note,
        })
doc = {"schema_version": 1, "labels": items}
with open("labels.json", "w", encoding="utf-8") as f:
    json.dump(doc, f, ensure_ascii=False, indent=2)
    f.write("\n")
print("校准集已生成：100 条媒体 + labels.json")
PY
