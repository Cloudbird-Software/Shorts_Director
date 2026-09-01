# evals/ —— 制式评估套件定义 + mock 商家数据集（版本化，IFACE-4）

E2 实验的输入面：套件（形态×模型×seed 集×断言包×预算）由
internal/eval 消费；mock 商家数据集供形态套件与端到端管线取材。

## 硬规则

- **禁止运行时网络抓取**（IFACE-4）：种子图要么是合成图
  （`gen_seed_images.sh`，ffmpeg lavfi 确定性生成，可安全入仓），
  要么登记公开图库 URL 清单（seed_images[].source.type=public_gallery
  - url + license，图不入仓，实验前人工落位）。
- **无真实个人身份信息/生物特征**（INV-6）：所有商家 fictional=true，
  电话/地址均为占位虚构；人像种子当前是合成占位，替换真实人像必须
  附授权记录落位 license_placeholder。
- 套件 JSON 经 eval.LoadSuite 受控校验（gen_form 词表 / IFACE-1 op
  枚举 / qc 断言白名单）；新增/改套件跑 `make go-check`
  （internal/eval/dataset_test.go 守护结构）。

## 布局

```
merchants/<slug>/merchant.json   信息表（店名/招牌项/价格/地址/电话/AIGC 文案）
merchants/<slug>/seed_*.png      合成种子图（hero=氛围空镜；portrait=数字人）
suites/form1_ambience.json       形态1 套件（V100 实测，3 商家 × K=5）
suites/form4_digital_human.json  形态4 套件（V100 实测，1 商家 × K=5）
suites/form1_smoke_fake.json     CI 冒烟（fake 后端 + golden fixtures）
gen_seed_images.sh               合成种子图生成器（重生成用）
```

## 注意

- form1/form4 套件的 model 字段是初始候选（DECISION-3：排序由 doctor +
  E1 冒烟裁决），E1 后换模型只改 suite 的 model/params，不改契约。
- 冒烟套件的条目请求与 testdata/golden/gen_i2v fixtures 逐字段咬合
  ——改动任一侧须同步另一侧（digest 键会变）。
