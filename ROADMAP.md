# Niuniu Education Roadmap

本文件记录已经确认、但当前尚未实现的功能演进、已知缺陷与体验改进。

状态说明：

* `待规划`：方向已经确认，尚未细化实现方案。
* `待实现`：方案基本明确，可以进入开发。
* `进行中`：已经开始开发。
* `已完成`：已经实现并通过验证，保留记录用于追溯。

## 发音系统

### P1：接入离线真人发音库（Cambridge + TFD）

状态：`已完成`（2026-07-12）

当前现状（改进前）：

* Composite Provider 只接入 Free Dictionary 和 Wiktionary 在线来源。
* Free Dictionary 英式录音覆盖率偏低；Wiktionary 需翻墙，国内云服务器部署不可靠。
* 在线来源均未命中时，前端用浏览器 Web Speech API（en-GB TTS）兜底，音色和清晰度明显弱于真人录音。

已完成的改进：

* 基于 thousandlemons/English-words-pronunciation-mp3-audio-download 的 `ultimate.json` 索引，批量预下载了 Cambridge（9,127 词）和 The Free Dictionary（42,028 词）的英式真人录音，共约 46,000 词、340MB，存放在项目 data 目录之外的独立语料库目录。
* 新增通用 `LocalCorpusProvider`（`platform/pronunciation/local_corpus.go`），按 `{word}.mp3` 查找本地音频，命中即用、零网络依赖。
* `provider.Result` 增加 `FilePath` 字段；`Service.resolveFresh` 检测到本地文件时直接拷贝到缓存目录，跳过 HTTP 下载。在线 provider 不填该字段，行为完全不变。
* Provider 链调整为 `cambridge → tfd → free_dictionary → wiktionary`：Cambridge 音质优先、TFD 覆盖兜底、在线来源作为最终兜底。
* 语料库根目录通过 `pronunciation.corpusDir`（yaml）或 `NIUNIU_PRONUNCIATION_CORPUS_DIR`（环境变量）配置；未配置时离线 provider 始终返回未找到，自动降级到在线来源。
* 修复 TFD 无 ID3 头 mp3 的 MIME 嗅探问题（`application/octet-stream` 退回到 `audio/mpeg`）。

完成标准验证：

* 三级回退实测通过：`factory`（Cambridge 有）→ cambridge；`middle`（TFD 有）→ tfd；`woman`（都没有）→ free_dictionary。
* 命中后音频缓存到 `data/audio/pronunciations/en-GB/`，后续播放纯本地。
* 离线库放在 data 目录之外，`reset-today.sh` 的清理流程不影响。

相关代码：`backend/internal/platform/pronunciation/local_corpus.go`、`backend/internal/platform/pronunciation/provider.go`、`backend/internal/module/pronunciation/service.go`、`backend/internal/config/config.go`、`backend/cmd/niuniu/main.go`、语料库目录 `$HOME/github/apodemakeles/niuniu-education-pronunciation-corpus/`

### P1：重构发音缓存身份模型

状态：`待规划`

当前现状：

* 数据库中的发音缓存以 `wordId + locale` 为主键。
* 磁盘文件名由小写单词、地区、Provider 和原始音频 URL 生成。
* 相同拼写如果对应不同 `wordId`，仍可能重复查询和下载。
* 还不能准确区分 `record` 这类同形异音词的不同读音。

目标方案：

* 使用 `normalizedText + locale + pronunciationVariant` 作为成熟版本的发音资源身份。
* `pronunciationVariant` 应能容纳词性、音标或人工指定版本。
* `wordId` 只负责关联词库记录与共享发音资源，不再直接决定音频资源是否重复。

完成标准：

* 相同单词、相同地区、相同发音版本只保存和下载一份音频。
* 同形异音词可以分别关联正确读音。
* 已有发音缓存能够安全迁移或重新构建。

### P1：在录入单词时预先准备发音缓存

状态：`待规划`

当前现状：

* 用户点击“听读音”后，系统才首次查询 Provider 并下载音频。
* 第一次播放可能受到上游响应时间和限流影响。

目标方案：

* 单词录入成功后，异步查询发音 Provider 并生成本地缓存。
* 录入流程不应同步等待音频下载，也不应因为发音服务失败而导致单词录入失败。
* 需要具备任务重试、并发限制、429 退避和失败状态查询能力。
* 点击播放时仍保留按需补偿逻辑，处理预生成失败或缓存文件丢失。

完成标准：

* 正常情况下，学生第一次点击“听读音”即可直接播放本地缓存。
* 发音服务不可用时不影响单词录入。
* 批量导入不会瞬间向外部 Provider 发起大量请求。

### P2：实现 Kokoro Provider

状态：`待规划`

当前现状：

* 离线真人发音库（Cambridge + TFD，约 46,000 词）已作为主力来源接入，命中即用、零网络依赖。
* 但离线库只覆盖约 46% 的索引词；离线库和 Free Dictionary 在线来源都未命中的词，仍由浏览器 `en-GB` TTS 兜底。
* 实测 `keep`、`put away` 未命中真人录音后使用了浏览器男声，清晰度和音色明显弱于真人录音。
* 国内云服务器不能可靠直连 Wiktionary/Wikimedia，正式部署不能把它作为运行时必须可用的在线依赖。

目标方案：

* 基于现有 Pronunciation Provider 接口实现 Kokoro Provider。
* 优先选择自然、标准的英式英语音色。
* Kokoro 仅处理真人词典录音缺失的单词，生成结果继续写入统一的本地缓存。
* Kokoro 接入后逐步取消浏览器 TTS 作为正式发音来源，仅保留为服务完全不可用时的临时兜底。
* 国内部署优先使用预生成的本地音频；Wiktionary 只作为构建期或具备合规稳定出口时的补充来源。
* 明确模型部署方式、硬件要求、生成速度、音色许可证和移动端使用方式。

完成标准：

* Kokoro 可以作为 Composite Provider 的可配置后备来源。
* 同一发音只生成一次，后续从本地缓存播放。
* 对一组小学常用词进行人工听感验收，不出现明显截断、尾音缺失或非目标口音。

## 延伸阅读短文生成

### P1：阅读完成按钮与后端停留时长校验不同步

状态：`已完成`（2026-07-12）

当前现状：

* `ReadingView` 由 `v-show` 控制显示，在今日任务加载后即已挂载；其本地计时器从组件挂载开始累计，而不是从学生进入阅读页、短文成功加载时开始。
* 后端只在 `GET /practice/reading` 返回成功短文时写入 `reading_started_at`，并以该时间校验最短阅读时长。
* `reading` 属性稍后从 `null` 更新为后端响应时，前端没有根据新的 `elapsedSeconds` 重置或同步本地计时，因此按钮会稳定地比后端提前开放；提前量大致等于学生在首页和单词卡阶段停留的时间。

目标方案：

* 前端倒计时以服务端返回的 `elapsedSeconds` 和实际进入阅读页的时刻为统一基准，不在隐藏状态提前累计。
* 阅读数据刷新、重新生成、离开后重新进入页面时，应重新与服务端时间同步，服务端继续作为完成资格的最终判断方。
* 后端仍作为完成资格的最终判断方；前端收到时长不足的拒绝后，立即重新获取阅读数据并校准倒计时。

完成标准：

* 无论学生在首页或单词卡停留多久，首次进入阅读页时前后端显示的已阅读时长误差不超过 1 秒。
* 前端按钮开放后立即提交可以通过后端校验，不再出现“按钮可点但继续阅读一会儿”的稳定复现问题。
* 增加覆盖组件延迟拿到 `reading` 数据、隐藏状态不计时、服务端拒绝后校准的自动化测试。

已完成的改进：

* 阅读组件新增显式 `active` 状态，只在阅读页可见且短文生成成功时推进本地显示时间。
* `reading` 数据从服务端更新时，本地倒计时根据 `startedAt + elapsedSeconds` 重新校准。
* 后端拒绝过早完成请求时，前端自动重新加载阅读数据，以服务端时间恢复倒计时。
* 新增 `ReadingView.test.ts`，覆盖隐藏挂载、服务端重校准和离开阅读页暂停三个场景。

相关代码：`frontend/src/views/PracticeView.vue`、`frontend/src/components/practice/ReadingView.vue`、`backend/internal/module/studentwordtask/service.go`

### P1：短文生成校验失败率偏高，部分目标词无法稳定复现

状态：`已完成`（2026-07-11）

已完成的改进：

* 校验策略从「恰好出现 2 次」放宽为「至少出现 2 次」（`reading.go` `ValidatePassage`），不再因 LLM 多写 1-2 次目标词而判失败。
* 覆盖词选择阶段过滤含空格或标点的多词短语（`reading.go` `isSingleWord`），如 `Nice to see you!`、`put away` 不再被选为覆盖词。
* 重试次数从 3 次提高到 5 次（`readingMaxRetry=4`，首次 + 4 次重试）。
* LLM 切换为 DeepSeek 官方 `deepseek-chat`（非推理模型，单次调用约 3s），替代原先经 SiliconFlow 中转的 `deepseek-ai/DeepSeek-V4-Pro`（推理模型，单次 60-180s 经常超时）。
* prompt 文案同步更新为「至少出现 2 次」。
* 设计文档 `design/docs/student-word-task.md` 已同步更新。

相关代码：`backend/internal/module/studentwordtask/reading.go`、`backend/internal/config/config.go`、`data/config.yaml`

### P2：短文生成 LLM 调用超时与 503 的容错

状态：`已完成`（2026-07-11）

已完成的改进：

* LLM 切换到 DeepSeek 官方直连（`https://api.deepseek.com`），不再经 SiliconFlow 中转，消除了 503 `System is too busy` 问题。
* `deepseek-chat` 单次调用约 3s（原先 60-180s），5 次重试总耗时约 15s（原先最坏 9 分钟）。
* 后端补全了短文生成全链路日志（`ensureReadingAsync` / `generateReading`），可区分「执行中」与「异常中断」。
* `GetReading` 增加 pending 过期自愈：僵尸 pending（无后台任务在跑且超过 5 分钟）自动触发同步重生成。
* 前端 `regenerate()` 修复为生成后自动轮询；pending 分支增加「重新生成」按钮。

相关代码：`backend/internal/module/studentwordtask/service.go`、`frontend/src/stores/practice.ts`、`frontend/src/components/practice/ReadingView.vue`

## 今日任务完成与打卡

### P1：AI 学习例句生成、批量进度与历史词补全

状态：`已完成`（2026-07-12）

已确认的目标：

* 家长逐个录入且确认保存后，为每个非重复单词生成 3 个适合小学阶段的 AI 英文例句；每句包含目标单词或短语，允许出现多次但不强制。
* 拍照导入和粘贴导入确认后，对新增词逐个生成例句，并在前端展示实时进度、单词级成功或失败状态。
* 家长可对任意单条例句重新生成；学生单词卡每次从该词的 3 条例句中随机展示 1 条。
* 词库增加主动“补全缺失例句”入口，不自动消耗模型调用；家长确认后以带进度的批量任务补全历史单词。

完成标准：

* 单词保存或批量确认时，重复词不触发例句生成；新增词有 3 条可管理的例句记录。
* 批量生成过程不阻塞页面、可看到当前进度；单条或单词失败不会中断其他词生成。
* 学生端不再显示系统占位例句，已生成词随机展示其一。

已完成的实现：

* 新增 `word_examples` 独立表和迁移 `006_word_examples.sql`，每词保存 3 条例句，可逐条重新生成。
* 使用现有 DeepSeek `deepseek-v4-flash` 配置生成例句；后端会校验 JSON、目标词或短语存在、句子不超过 12 个英文词、三句不重复，并在不合格时自动重试。
* 逐个录入保存后立即展示生成动画与三条例句；拍照/粘贴导入确认后通过 SSE 展示逐词进度，单词失败不影响其余单词。
* 词库增加“补全缺失例句”入口；学生端每次读取单词卡时随机选取一条已生成例句，缺失时才保留兼容占位。

相关代码：`backend/internal/module/wordlibrary`、`backend/internal/module/studentwordtask`、`frontend/src/components/word`、`frontend/src/components/import`

### P1：学习任务状态与单词库状态分离，家长页无法反映实际学习进度

状态：`已完成`（2026-07-12）

当前现状：

* 学生端“不会读”、首次默写正确/错误会更新 `word_learning.learning_status`、成功次数和下次到期日期，任务生成也以这些字段为准。
* 家长端单词库列表和状态筛选读取的是 `words.status`；学生学习流程不会更新该字段。
* 因此同一个词可能在学生任务中已是“需强化”，但家长单词库仍显示旧状态，形成两套状态来源。

目标方案：

* 已明确 `word_learning` 为实际学习状态的唯一来源；单词库列表和状态筛选联结该记录。
* 编辑词义、音标、词性仍保留在 `words`，不再允许编辑页直接改学习状态。
* 无学习记录的新词展示为“未学”；逐个录入时选择“需强化词”会立即创建 `reinforce` 学习记录并进入复习计划。
* 已添加安全迁移：仅把缺少学习记录的历史非未学状态迁入 `word_learning`，不覆盖已有学习结果。

完成标准：

* 学生端标记“不会读”或默写错误后，家长单词库刷新即可看到“需强化”。
* 学习状态筛选、逐个录入的需强化词与明日任务生成使用同一份状态数据。
* 不存在同一词在两个页面显示相互矛盾学习状态的情况。

相关代码：`backend/internal/module/wordlibrary/store.go`、`backend/internal/module/studentwordtask/store.go`、`backend/internal/platform/storage/migrations/004_unify_word_learning_status.sql`、`frontend/src/components/word/WordCreateModal.vue`

### P2：新词已学完且无到期复习时，缺少明确的“休息日”反馈

状态：`待实现`

当前现状：

* 当词库仍有单词、但新词池为空且非新词池没有到期候选时，今日任务会生成 0 个词。
* 学习首页仅禁用按钮并显示“暂无任务”；阅读生成会因没有任务词失败，孩子无法知道这其实是正常的复习间隔，而不是系统故障。

目标方案：

* 将该状态明确呈现为“今日复习完成，可以休息”；显示下一次到期复习日期，或提示家长可补充新词。
* 不创建无任务的阅读流程，不把正常休息日标记为生成失败。

完成标准：

* 新词全部进入学习流程且当天无到期复习时，孩子能看懂无需学习的原因。
* 页面不显示失败状态，也不会触发无任务短文生成。

相关代码：`backend/internal/module/studentwordtask/service.go`、`frontend/src/components/practice/TodayDashboard.vue`

### P1：新词随机抽取与静态复习容量会造成长期排队风险

状态：`已完成`（2026-07-12）

已完成的改进：

* 新词由完全随机改为按入库时间公平排队，保证最久未学的词优先进入任务。
* 当逾期复习词达到 3 个时，新词收紧为 1 个；达到 6 个时暂停新词，先回收复习积压。
* 新词未使用的名额转给需强化词：基础复习容量为 9 个，可动态补至最多 12 个，但每日总量不超过原先的 3 新词 + 9 复习词。
* 短期巩固由“3 次成功后 60 天”调整为连续首次默写正确后的 `1 → 3 → 7 → 14` 天间隔；累计第 5 次成功后进入 `30 → 60 → 120 → 240` 天长期抽查。
* 首次默写结果和订正规则不变：订正用于纠错与完成任务，不计入有效成功次数。

相关代码：`backend/internal/module/studentwordtask/scheduler.go`、`backend/internal/module/studentwordtask/review.go`、`backend/internal/platform/storage/migrations/005_learning_queue_fairness.sql`

### P2：新排程上线后需要重新进行 180 天模拟验证

状态：`待实现`

当前现状：

* 既有模拟脚本和结果基于旧的随机新词、3 次成功后进入 60 天抽查的规则。
* 当前已改为公平排队、复习积压收紧新词、空位转给需强化词，以及 5 次短期巩固后才进入长期抽查；旧结果不能代表新算法的学习量和遗忘风险。

目标方案：

* 同步模拟脚本的队列规则与 `1 → 3 → 7 → 14 → 30 → 60 → 120 → 240` 天间隔。
* 用固定种子输出 180 天内的待学最久天数、逾期词数、每日任务量、短期和长期复测正确率，并设置可验收的上限。

完成标准：

* 模拟规则与后端排程函数一致，能复现新词名额转让和逾期暂停新词。
* 文档中的推荐参数和模拟结论来自新规则的实际运行结果。

相关代码：`design/scripts/simulate-word-schedule.mjs`、`design/docs/student-word-task.md`

### P1：漏做任务缺少显式处理，打卡接口未在后端校验完成状态

状态：`待实现`

当前现状：

* 当天未完成的 `daily_tasks` 会保留在历史表中；次日按当前学习状态生成新任务，孩子端没有“昨天未完成”提示或补做入口。
* 前端只有在完成页才展示打卡按钮，但 `POST /practice/checkin` 后端目前可直接写入打卡记录，未验证当天首次默写与订正是否均已完成。

目标方案：

* 后端打卡前必须调用当天完成校验；未完成时返回明确错误，不能生成打卡记录或连续天数。
* 首页区分“今日待完成”和“昨日漏做”：不重写昨天首次结果，但向家长/孩子明确展示漏做状态，并提供符合产品决定的补做或跳过规则。

完成标准：

* 绕过前端直接请求打卡接口不能给未完成任务打卡。
* 漏做一天后，次日任务与连续打卡的影响在界面上可理解、可追溯。

相关代码：`backend/internal/module/studentwordtask/service.go`、`frontend/src/views/PracticeView.vue`

### P1：学生回看默写时暴露首次错误拼写，可能强化错误印象

状态：`已完成`（2026-07-12）

当前现状：

* 今日任务完成后，学生仍可从步骤导航回到“默写”页。
* 页面把每个锁定题目的 `firstAnswer` 填入只读输入框；首次写错的拼写会和正确答案一样被完整展示。
* 首次默写答案对家长复盘有价值，但对学生的完成后回看会增加错误拼写的重复暴露。

已完成的改进：

* 任务完成后，学生端步骤入口由“默写”变为“今日复盘”。
* 复盘页改用静态正确写法卡片，不再展示只读输入框；首次错误拼写不会回放给学生。
* 首次正确显示“一次写对”，订正成功显示“后来订正正确”；两者都只显示正确写法。
* 原始首次答案和订正记录继续保留在数据层，供家长端后续复盘使用；新增组件测试覆盖学生端不展示首次错误拼写。
* 重新开始今日任务时，前端会清空上一轮的完成页数据和已解锁步骤，避免导航提前误显示“今日复盘”。
* 首页根据后端返回的当日完成状态显示“今日已完成，查看复盘”，刷新后也不会误导学生重新开始并回放历史默写记录。

相关代码：`frontend/src/views/PracticeView.vue`、`frontend/src/components/practice/DictationView.vue`、`frontend/src/components/practice/__tests__/DictationView.test.ts`

### P2：单词卡因学习队列过长而被拉伸，重点内容超出首屏

状态：`已完成`（2026-07-12；同日按体验反馈调整为页面整体滚动）

当前现状：

* 桌面端单词卡与学习队列使用同一 Grid 行，默认拉伸规则会让左侧单词卡随右侧队列长度增长。
* 左卡使用纵向分散布局，卡片被拉长后，音标、单词、释义、例句和操作区之间出现过大的空白；单词和音标字号也偏大。

已完成的改进：

* 收紧左侧字号与间距（单词最大 76px、音标 22px），并去掉 `justify-content: space-between`，避免栏内被拉开空白。
* 两栏用 `align-items: start` 顶对齐；不再给左右栏设固定高度或内部滚轮，内容顺延增高，由页面整体滚动。
* 学习队列随词条数量自然增高，不出现独立滚动条。

相关代码：`frontend/src/assets/kid.css`

### P1：完成页打卡按钮未触发打卡流程

状态：`已完成`（2026-07-12）

当前现状：

* 完成页按钮曾绑定未定义的 `onFinish`，点击后不会向父组件发出 `finish` 事件，因此 `POST /practice/checkin` 不会被调用。
* 打卡成功与重复打卡没有明确区分，且请求失败时缺少学生端反馈。

已完成的改进：

* `DoneView` 通过 `emit('finish')` 接入既有的打卡事件链路。
* 打卡完成后刷新完成页打卡数据；首次成功提示“打卡成功！”，连续第 7 天仍展示 bonus 奖励。
* 当天已打卡时按钮显示“今日已打卡”并禁用；请求进行中显示“正在打卡…”，避免重复提交。
* 打卡失败时显示可操作的失败提示；新增组件测试覆盖事件发出与已打卡禁用状态。

相关代码：`frontend/src/components/practice/DoneView.vue`、`frontend/src/views/PracticeView.vue`、`frontend/src/stores/practice.ts`、`frontend/src/components/practice/__tests__/DoneView.test.ts`

### P1：完成页“再练一遍错词”未进入可订正状态

状态：`已完成`（2026-07-12）

当前现状：

* 完成页点击“再练一遍错词”仅切换回默写页面，并将页面级 `submitted` 状态重置为 `false`。
* 默写题目的首次结果已经由后端锁定；在 `submitted=false` 时，前端不会初始化错词订正目标，也不会进入可编辑的订正模式，孩子只能看到锁定输入框和“查看结果”。

已完成的改进：

* 完成页显式请求订正模式；默写页根据首次默写结果重新筛选目标，只展示首次错误或未填写的词，并清空其输入框。
* 订正请求只提交这些错词；成功后返回完成页，并保留首次默写统计。
* 后端 `CorrectDictation` 仅写入 `correction_*` 字段，不会改写 `first_dictation_*`、学习状态、有效成功次数或下次复习日期。
* 新增组件测试，覆盖从完成页进入订正后只显示错词，以及仅提交错词的新答案。

相关代码：`frontend/src/views/PracticeView.vue`、`frontend/src/components/practice/DictationView.vue`、`backend/internal/module/studentwordtask/service.go`、`frontend/src/components/practice/__tests__/DictationView.test.ts`

### P1：错词订正未作为进入完成页的必经步骤

状态：`已完成`（2026-07-12）

当前现状：

* 学生首次提交默写后，前端无论是否存在错误或未填写，都会立即进入完成页。
* 完成页提供“再练一遍错词”作为可选入口，学生可以不订正而直接点击“完成今日任务”打卡。
* 这与设计文档规定的“首次提交 → 订正错误词 → 进入今日完成页”顺序不一致。

已完成的改进：

* 首次全部正确时，仍直接进入完成页。
* 存在错误或未填写时，自动停留在默写页并进入订正模式；完成页步骤不会提前解锁。
* 每个错词的订正结果都必须为正确，才进入完成页并开放打卡；仍错误或未填写的词会保留在订正页继续练习。
* 顶部步骤导航的“查看结果”入口也会拦截任何尚未订正正确的错词。
* 订正接口仅更新本轮实际提交的词，已订正正确的词不会被后续请求当作空答案覆盖。
* 新增前端测试，覆盖首次默写提交不会提前解锁完成页，以及订正错误词继续展示的场景；后端测试覆盖分轮订正时已正确词的结果保持不变。

相关代码：`frontend/src/views/PracticeView.vue`、`frontend/src/stores/practice.ts`、`frontend/src/stores/__tests__/practice.test.ts`、`backend/internal/module/studentwordtask/service.go`、`backend/internal/module/studentwordtask/service_test.go`

## 开发与调试

### P2：调试模式跳过延伸阅读最短停留

状态：`已完成`（2026-07-12）

当前现状：

* 延伸阅读默认需停留满 `reading_min_seconds`（通常 120 秒）才能点击「我读完短文了」。
* 本地联调、家长自测走完整流程时，等待成本过高。

已完成的改进：

* `data/config.yaml` 增加 `debug.enabled`；也可用环境变量 `NIUNIU_DEBUG=true` 覆盖。
* 开启后，阅读接口立即返回 `canFinish=true` / `debugMode=true`，完成接口跳过最短停留校验。
* 启动日志会打印调试模式告警；正式给孩子使用前应关闭。

完成标准：

* `debug.enabled: false`（默认）时行为与原先一致。
* `debug.enabled: true` 时进入阅读页即可完成，无需等满时长。
* 重启服务后配置生效。

相关代码：`backend/internal/config/config.go`、`backend/cmd/niuniu/main.go`、`backend/internal/module/studentwordtask/service.go`、`frontend/src/components/practice/ReadingView.vue`、`design/docs/architecture.md`
