#!/bin/sh
# 合成种子图生成器（IFACE-4：禁止运行时网络抓取；INV-6：无真实个人身份信息）。
# 用 ffmpeg lavfi gradients 确定性合成 9:16 渐变图——零第三方依赖，
# 同参数恒同输出。公开图库来源清单日后按需登记进各 merchant.json 的
# seed_images[].source（type: public_gallery + url + license），不入仓大文件。
set -eu
cd "$(dirname "$0")"

gen() { # gen <out> <c0> <c1> <speed>
  ffmpeg -y -loglevel error -f lavfi \
    -i "gradients=s=576x1024:c0=$2:c1=$3:speed=$4" -frames:v 1 "$1"
}

gen merchants/noodles_lanjie/seed_hero.png 0x8B1A1A 0x1A0E08 0.001
gen merchants/valley_coffee/seed_hero.png 0x3E2B1F 0x0E1418 0.001
gen merchants/yueyan_beauty/seed_hero.png 0x6C3B7A 0x140E18 0.001
# 形态4 人像照（数字人口播的口型宿主；合成渐变无真实生物特征——INV-6）
gen merchants/noodles_lanjie/seed_portrait.png 0x5A2A3C 0x12101A 0.0008
gen merchants/valley_coffee/seed_portrait.png 0x2E3C4A 0x0E1218 0.0008
gen merchants/yueyan_beauty/seed_portrait.png 0x2A3B6C 0x101418 0.0008

echo "种子图已生成（合成数据，可安全入仓）"
