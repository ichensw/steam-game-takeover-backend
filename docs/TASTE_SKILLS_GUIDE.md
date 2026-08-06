# Taste Skill 使用手册

更新时间：2026-08-06

这份手册记录当前工作区可用的视觉设计、界面实现、图像驱动设计相关 Skill。这里的 "使用" 指在给 Codex 下需求时明确提及 Skill 名称和目标。Skill 不是 npm 包，也不是在项目内执行的一条命令；它是一套约束、决策方法和交付标准。

## 1. 先理解使用方式

### 1.1 一句话规则

先选一个主 Skill，再按需要补一个配套 Skill。不要把多个审美主导型 Skill 全部堆到同一个页面上。

推荐的请求结构：

```text
用 `design-taste-frontend` 设计并实现这个页面。
产品：...
用户：...
页面的唯一目标：...
风格：...
已有资产/参考：...
必须保留：...
不要：...
```

对于现有页面，补充：

```text
这是增量改造，不要换技术栈，不要改变现有信息架构；
先按 `redesign-existing-projects` 做审计，再实施高优先级问题。
```

### 1.2 优先级

1. 明确的产品需求、现有设计系统、无障碍、性能、业务功能优先。
2. 当前项目已有的组件库、图标库、样式体系优先。
3. Codex 的系统/开发规范优先于某个 Taste Skill 的偏好。
4. Taste Skill 负责方向、取舍和质量门槛，不能用来合理化功能缺失、不可访问或性能差。

例如：某些 Taste Skill 偏好特定图标库，但已有项目若统一使用 Lucide，则保持现有一致性；不能为了视觉偏好把全项目图标替换一遍。

### 1.3 选型速查

| 你的任务 | 首选 Skill | 常见配套 | 不要优先用 |
| --- | --- | --- | --- |
| 新营销站、作品集、品牌落地页 | `design-taste-frontend` | `frontend-design`、`imagegen-frontend-web` | `frontend-ui-engineering` 单独主导审美 |
| 现有网站视觉升级 | `redesign-existing-projects` | `design-taste-frontend` | 直接全量重写 |
| SaaS、后台、表单、业务工具 | `frontend-ui-engineering` | `design-taste-frontend` 的少量反模板规则 | `gpt-taste`、`high-end-visual-design` |
| 有截图/设计稿，要高保真落地 | `image-to-code` | `frontend-ui-engineering` | 凭描述重新发明布局 |
| 需要生成 Web 页面视觉资产或视觉方向 | `imagegen-frontend-web` | `image-to-code` | 只用 CSS 假装有真实视觉资产 |
| 需要设计移动端 App 多屏体验 | `imagegen-frontend-mobile` | `frontend-ui-engineering` | Web 落地页 Skill 直接套用 |
| 品牌 Logo、品牌板、视觉识别提案 | `brandkit` | `imagegen-frontend-web` | 直接开始写产品页面 |
| Awwwards/创意工作室级营销页面 | `gpt-taste` 或 `high-end-visual-design`，二选一 | `imagegen-frontend-web` | 企业后台/管理台 |
| 极简、克制的产品/工具界面 | `minimalist-ui` | `frontend-ui-engineering` | 高运动、高装饰 Skill |
| 工业、终端、战术遥测、硬核工具视觉 | `industrial-brutalist-ui` | `frontend-ui-engineering` | 品牌板、奢侈品风格 |
| 要给 Google Stitch 建立长期可复用的视觉规范 | `stitch-design-taste` | `design-taste-frontend` | 直接产出 React 代码 |

## 2. 推荐组合与禁忌

### 2.1 可以组合

| 主 Skill | 配套 Skill | 分工 |
| --- | --- | --- |
| `design-taste-frontend` | `frontend-design` | 前者负责可执行的前端规则，后者帮助从产品主题中形成独特视觉主张。 |
| `design-taste-frontend` | `redesign-existing-projects` | 前者定方向，后者先审计、确定低风险改造顺序。 |
| `image-to-code` | `frontend-ui-engineering` | 前者从截图提取视觉和结构，后者保证组件、状态、无障碍和响应式可上线。 |
| `imagegen-frontend-web` | `image-to-code` | 先产出或确认真实页面视觉资产，再将画面转为代码。 |
| `imagegen-frontend-mobile` | `frontend-ui-engineering` | 前者覆盖多屏艺术方向和平台语义，后者落实真实交互状态。 |
| `brandkit` | `imagegen-frontend-web` | 先有 Logo、色彩、字体和影像语言，再做品牌网站。 |
| `stitch-design-taste` | `design-taste-frontend` | 前者产出 Stitch 可读的 `DESIGN.md`，后者给 Web 实施提供反模板质量线。 |

### 2.2 不建议同时主导

| 组合 | 原因 | 正确做法 |
| --- | --- | --- |
| `gpt-taste` + `high-end-visual-design` | 都是高表现力营销页方法论，容易叠加过度动画和组件特效。 | 只选一个作为主风格引擎。 |
| `minimalist-ui` + `industrial-brutalist-ui` | 一个要求安静、实用、低装饰，另一个强调工业印刷和终端张力。 | 选择其中一个；不要折中成无主题拼贴。 |
| `design-taste-frontend` + `design-taste-frontend-v1` | v1 是旧版高主动性规则，新版已经加入更完整的 Brief 解读、设计系统映射和改造协议。 | 默认使用新版；仅在要复现旧项目风格时使用 v1。 |
| `image-to-code` + "自由发挥重设计" | 高保真还原和创意重设计是两类目标。 | 先决定是 `忠实还原` 还是 `参考后重设计`。 |
| 任何 Awwwards Skill + 高密度后台 | 高运动、非对称和大面积视觉叙事会伤害扫描与重复操作效率。 | 后台使用 `frontend-ui-engineering`，只在入口页或空状态局部借用视觉语言。 |

## 3. 通用工作流

### 3.1 新页面

1. 说清产品、用户、页面唯一目标和限制。
2. 让 Agent 给出一行 `Design Read`：页面类型、受众、视觉语言和实现基础。
3. 决定三个设计拨盘：`DESIGN_VARIANCE`、`MOTION_INTENSITY`、`VISUAL_DENSITY`。
4. 选一个主 Skill；若有图像资产，再补图像 Skill。
5. 实现后检查桌面与移动端、键盘操作、空/加载/错误状态和真实内容长度。

常用请求：

```text
用 `design-taste-frontend` 做一个游戏接龙小程序官网。
读作：面向游戏群组织者的轻量营销页，偏轻松但可信，目标是下载/打开小程序。
设计拨盘：variance 6、motion 3、density 4。
不要紫色 AI 渐变、三等分功能卡、营销套话；保留现有品牌色和 Logo。
```

### 3.2 现有项目改造

1. 先声明改造模式：`preserve`（保留品牌和信息架构）或 `overhaul`（允许重构布局）。
2. 用 `redesign-existing-projects` 做审计：字体、色彩、布局、状态、内容、组件、图标和代码质量。
3. 按低风险高收益顺序改：字体 -> 色彩 -> Hover/Active -> 间距/布局 -> 通用组件 -> 状态补齐。
4. 只处理本任务涉及的页面，不把全站重做伪装成一个小优化。

常用请求：

```text
用 `redesign-existing-projects` 审计并改造这个 React 页面。
这是 preserve 模式：保留 Ant Design、接口与信息架构，只优化层级、密度、移动端和状态。
先给出问题清单和优先级，再实施 P0/P1。
```

### 3.3 参考图或截图还原

1. 指定参考图是 `严格还原`、`保留结构换品牌`，还是 `只借鉴氛围`。
2. 用 `image-to-code` 提取布局、文字层级、间距、组件形态、媒体裁切和颜色关系。
3. 有缺失细节时，先补生成/提取图像，不要用凭空编造的 CSS 填满。
4. 把图像放进固定媒体框，响应式时改变排版，不随意拉伸或裁切原图。

常用请求：

```text
用 `image-to-code` 严格还原这张截图的首屏和两个内容区。
保留信息层级与留白比例，不复制品牌 Logo 和文案；替换成我的真实资产。
输出前检查桌面 1440 和移动 390 的首屏构图。
```

## 4. Skill 详解

### 4.1 `design-taste-frontend`

**定位**：新版主力 Taste Skill。用于落地页、作品集、编辑型页面和网站改造；明确不以数据表、仪表盘、多步骤业务 UI 为主要目标。

**适用场景**：

- 新品牌官网、产品营销页、创意工作室页、个人作品集。
- 有模糊风格词或参考链接，需要先判断真正方向。
- 需要避免“深色紫蓝渐变 + 居中 Hero + 三张等宽卡片”的模板感。

**输入必须给什么**：产品、受众、页面类型、页面目标、品牌资产/参考、是否保留现有结构。若信息足够，Agent 应直接做判断；若存在明显分歧，只问一个关键问题。

**核心方法**：

- 先输出一行 `Design Read`，例如“面向技术采购者的 B2B 营销页，克制、可信，接近 Primer/Linear 语言”。
- 设置三拨盘，默认值为 `variance 8 / motion 6 / density 4`。极简、公共服务、Awwwards、编辑风等都有不同建议区间。
- 先映射到真实设计系统：Fluent、Material、Carbon、Polaris、Atlassian、Primer、GOV.UK、USWDS、Bootstrap、Radix、shadcn 等；同一项目只能有一个主系统。
- 视觉偏好不是设计系统时，用 CSS/现有组件库实现，并明确这只是“风格近似”，不是官方实现。

**硬性质量线**：

- 首先遵从主题与受众，不能默认 AI 紫色、泛滥玻璃、均分三卡、深色网格、假数据和模板文案。
- 使用真实或语义明确的内容；避免 Lorem Ipsum、Acme、Jane Doe、99.99% 之类占位痕迹。
- 响应式优先，使用 Grid 而不是复杂 Flex 百分比计算；全屏首屏优先 `100dvh`，避免移动端 `h-screen` 跳动。
- 动画使用 `transform` 和 `opacity`，支持 `prefers-reduced-motion`；不能为了“酷”牺牲性能。
- 交付前检查焦点、对比度、状态、移动端、暗色模式（适用时）、DOM 成本和真实链接。

**常用请求**：

```text
用 `design-taste-frontend`，先给一行 Design Read 和三拨盘，再实现。
这是一个面向独立游戏玩家的活动页，不是后台；风格要温暖、直接、像真实社区。
```

**不要用在**：密集运营后台、数据表和复杂多步骤表单。此类任务应以 `frontend-ui-engineering` 为主。

**原始说明**：`.agents/skills/design-taste-frontend/SKILL.md`

### 4.2 `design-taste-frontend-v1`

**定位**：旧版 `High-Agency Frontend Skill`。保留了三拨盘、反 AI 模板、创意组件库和 Motion Bento 思路，但没有新版完整的 Brief 解读、官方设计系统映射、重设计协议和更严谨的交付门禁。

**什么时候用**：

- 已有页面明显按旧版方式建成，想维持它的审美和动效节奏。
- 需要旧版的 `Motion-Engine Bento` 作为非常明确的视觉方向。

**什么时候不要用**：新项目。默认用新版 `design-taste-frontend`，避免同一页同时受两个版本的规则影响。

**请求示例**：

```text
这个页面延续旧项目的 Motion-Engine Bento 语言，用 `design-taste-frontend-v1`。
限制为首屏和展示区，后台表格不要套用这套动效。
```

**原始说明**：`.agents/skills/design-taste-frontend-v1/SKILL.md`

### 4.3 `frontend-design`

**定位**：通用设计总监视角。它不是实现规范，而是帮助页面从真实主题、受众和单一任务里形成有理由的视觉身份。

**适用场景**：

- 需求只有“做高级一点”“别像模板”，但没有明确风格。
- 需要先做设计方向，而不是立刻写组件。
- 需要从产品的材料、场景、文化和对象中提取视觉隐喻。

**核心方法**：

- 先明确具体主题、对象、受众与页面唯一任务。
- 做两轮：先给颜色、字体、布局、记忆点组成的紧凑设计计划；再自查是否只是默认审美。
- 用一个真正值得的“签名元素”承载胆量，其余部分克制，不让每个区块都在抢戏。
- 文案应服务理解和操作，使用主动语态、句式一致的命名、可行动的错误与空状态。

**最好的搭配**：和 `design-taste-frontend` 一起使用，前者提供创意方向，后者落到系统、布局和工程守则。

**原始说明**：`.codex/skills/frontend-design/SKILL.md`

### 4.4 `frontend-ui-engineering`

**定位**：生产级 UI 工程主入口。适用于用户实际反复操作的产品界面，而不是追求强视觉叙事的营销页。

**适用场景**：

- 后台、配置页、管理端、表单、筛选、数据展示、工作流页面。
- 新增或修改任何真实交互组件。
- 需要补齐 Loading、Error、Empty、移动端、键盘操作和状态管理。

**核心方法**：

- 组件做单一职责，数据获取与展示分开；优先组合，而不是给一个组件塞几十个配置项。
- 状态选择从简单到复杂：本地 state -> 提升 state -> Context -> URL -> Server state -> 全局 store。
- 遵守项目已有设计系统、间距、语义色和字号层级，不自由发明 13px、2.3rem 之类离散值。
- WCAG 2.1 AA：原生按钮优先、可键盘访问、表单有 Label、状态不是只靠颜色、焦点处理正确。
- 以 320/768/1024/1440 宽度验证；必要时用骨架屏和乐观更新改善感知性能。

**常用请求**：

```text
用 `frontend-ui-engineering` 改造这个后台配置页。
保持现有组件库和 API，不做营销页风格；补齐加载、错误、空状态、键盘操作和 320px 响应式。
```

**原始说明**：`.codex/skills/frontend-ui-engineering/SKILL.md`

### 4.5 `redesign-existing-projects`

**定位**：现有项目的审计和低风险升级 Skill。重点不是重新发明，而是先找到最影响品质的真实问题。

**审计维度**：字体、色彩与表面、布局、交互状态、文案、组件模式、图标、代码质量，以及 AI 常遗漏的法务链接、返回路径、404、表单校验、跳转链接和跳过主内容链接。

**推荐改造顺序**：

1. 换/校准字体。
2. 收敛调色板。
3. 补 Hover、Active、Focus 状态。
4. 修网格、宽度、间距和移动端。
5. 替换模板化组件模式。
6. 补加载、空、错误状态。
7. 做最后的字号和间距打磨。

**边界**：不换技术栈、不破坏功能、先检查依赖和 Tailwind 版本、每一步可验证。适合渐进升级，不适合借“改造”之名全站重写。

**原始说明**：`.agents/skills/redesign-existing-projects/SKILL.md`

### 4.6 `gpt-taste`

**定位**：Awwwards 级高创意营销页方法论，强调突破重复审美、AIDA 叙事、极简 Hero、无缝 Bento、GSAP 动效和 Hover 物理感。

**适用场景**：

- 创意工作室、品牌发布、作品集、活动主视觉页。
- 用户明确要求实验性、动效、Awwwards 感，且允许投入图像与动效制作成本。

**使用要点**：

- 先输出设计计划，再构建；不要直接堆特效。
- Hero 通常遵守两行表达约束，保持核心信息可读。
- 使用高质量真实资产，不用 Emoji、通用紫色渐变、三列卡片、虚构 KPI 或模板评价。
- Motion 必须有节奏和性能预算，不能对所有元素无差别地加动画。

**风险**：不适合数据密集型产品、低端设备优先场景、短周期小改动。它的“高差异”不等于“适合业务系统”。

**原始说明**：`.agents/skills/gpt-taste/SKILL.md`

### 4.7 `high-end-visual-design`

**定位**：另一套 Awwwards 级视觉与动效编排方法。通过选择一种氛围/纹理原型和一种布局原型，避免每次都产出同一套网页。

**适用场景**：

- 高端品牌、创意展示、需要“材质感”和强视觉入口的页面。
- 明确希望 Fluid Island 导航、磁吸按钮、滚动入场等微交互的营销页。

**核心要点**：

- 先选一个 Vibe/Texture，再选一个 Layout Archetype，不能把所有原型混合。
- 可使用双边框/嵌套边界、岛式 CTA、空间张力等细节，但只应服务层级。
- 动画只使用硬件友好的属性，给出降级策略和性能边界。
- 产出前检查是否出现圆角泛滥、霓虹发光、无理由渐变、居中模板 Hero 等绝对零容忍模式。

**与 `gpt-taste` 的选择**：偏叙事与 Bento/GSAP 选 `gpt-taste`；偏材质、组件触感和动效编排选本 Skill。二选一。

**原始说明**：`.agents/skills/high-end-visual-design/SKILL.md`

### 4.8 `image-to-code`

**定位**：图像优先的网站设计转代码 Skill。用于设计稿、截图、参考网站画面，或需要靠真实图像资产建立视觉品质的 Web 项目。

**适用场景**：

- 用户提供截图，要忠实还原结构、媒体、文字层级、色彩和间距。
- 用户没有截图但产品必须依赖真实视觉资产，不能靠纯 CSS 装饰完成。
- 页面有多个重要区块，需按区块生成/使用足量图像并保持一致性。

**核心方法**：

- 先做干净图像分析：区块、视觉锚点、字体、留白、媒体比例、按钮与组件细节。
- 明确“严格还原/保留结构换品牌/只借氛围”三种模式。
- 先获取足够图像再编码；旧图缺细节时重新生成，不裁切旧图硬凑。
- 媒体放在稳定的固定比例框里，图片不能拉伸变形，手机端不能只靠裁切来修布局。
- 文本、字体、间距、按钮、颜色应从参考中提取，不要用模板 CSS 替代观察。

**常用请求**：

```text
用 `image-to-code`。这是严格还原模式：依据我提供的三张截图实现，
优先还原首屏构图、图片比例、字重和区块节奏；缺失内容用同一视觉体系补齐。
```

**不要用在**：只有一句需求、没有图像方向、又不希望生成任何视觉资产的普通后台页面。

**原始说明**：`.agents/skills/image-to-code/SKILL.md`

### 4.9 `imagegen-frontend-web`

**定位**：Web 页面图像艺术指导 Skill。它不是“随便生成一张 Hero 图”，而是定义一个页面乃至整套站点的图像数量、构图、主题、版式、色彩和跨区块连续性。

**适用场景**：

- 品牌官网、产品发布、场地/对象/人物介绍页必须依赖可观察的真实视觉内容。
- 需要生成多个视觉区块，而不是只在 Hero 放一张背景图。
- 要先确定设计方向，再做图像生成和代码落地。

**核心方法**：

- 先确定 Theme、Background Character、Typography Character、Hero Architecture、Section System、Signature Components、Motion Language、Narrative Spine。
- 每一个重要区块都有明确构图锚点和背景模式；根据页面规模选择 4/8/12 区块图像包。
- Hero 先干净、信息少、主角明确，避免把文字压进复杂图片导致不可读。
- 维持同一站点多图一致性，同时避免每张图是同一种构图和同一套色调。
- 约束 AI 常见问题：无主题大渐变、卡片堆、假 KPI、轮播/跑马灯滥用、文字过大、缺乏第二阅读点。

**常用请求**：

```text
用 `imagegen-frontend-web` 为这个品牌站建立 8 个区块的图像方向，
先给视觉叙事和每个区块的画面用途，再生成/选择资产并实现页面。
```

**原始说明**：`.agents/skills/imagegen-frontend-web/SKILL.md`

### 4.10 `imagegen-frontend-mobile`

**定位**：移动 App 多屏视觉方向 Skill。强调多个连贯屏幕、平台语义、状态栏安全区、导航逻辑和不同业务类别的移动端偏置。

**适用场景**：

- 要生成或设计一套 iOS、Android、跨平台移动 App 界面。
- 需要 Onboarding、列表、详情、表单、状态页等完整屏幕流，而不是一张孤立 Mockup。

**开始前必须决定**：`iOS-native premium`、`Android-native premium` 或 `Cross-platform premium neutral`。不要把三种系统控件语言混用。

**核心方法**：

- 先规划完整用户流程，再生成足够屏幕；屏幕之间共享颜色、字体、组件和信息架构。
- 每个屏幕遵从安全区、系统区域、44px 触控目标、移动端导航和首屏清洁度。
- 默认包含设备 Mockup 仅用于展示场景，真实产品截图不应被手机壳遮挡关键信息。
- 针对金融、健康、生产力、社交、商业、生活方式等类别使用不同密度和交互偏置。
- 不能把桌面网页缩成手机，也不能让每屏都变成卡片堆和渐变背景。

**常用请求**：

```text
用 `imagegen-frontend-mobile` 设计 iOS-native premium 的活动接龙 App。
先给 6 个核心屏幕及导航流，再生成一致的屏幕方向；保持轻量、可快速发起活动。
```

**原始说明**：`.agents/skills/imagegen-frontend-mobile/SKILL.md`

### 4.11 `brandkit`

**定位**：品牌图像生成和品牌板系统。输出的是品牌基础，而不是直接代替产品 UX。

**适用场景**：

- 新品牌需要 Logo 概念、颜色、字体、影像方向、数字/实体应用示例。
- 需要为官网、海报、社交媒体或产品建立一致的视觉 DNA。

**核心方法**：

- 先做品牌策略：受众、品类、定位、气质、反差和禁忌。
- Logo 可从字母组合、产品动作、隐喻融合、负形和几何构造五类方法中选择一种主方法。
- 默认品牌板可使用 3 x 3：Logo Cover、构造、数字应用、品牌本质、色彩、字体、实体应用、影像方向、系统细节。
- 可选择 Dark Developer、Dark Product、Dark Nature、Security、Light Editorial、Luxury、Voice、Cultural Experimental 等视觉模式，但全套必须一致。
- 严格控制色彩、文字、Mockup、影像方向，不能生成泛化 Logo、无意义标语或混乱的品牌板。

**常用请求**：

```text
用 `brandkit` 为“兔兔窝”做一套偏游戏社群但不幼稚的品牌板。
输出品牌策略、3 个 Logo 概念方向、最终 3x3 品牌板、色彩/字体/图像规范。
```

**原始说明**：`.agents/skills/brandkit/SKILL.md`

### 4.12 `minimalist-ui`

**定位**：高级实用主义极简 UI。强调清晰、低噪音、温和单色基底、少量点缀色、明确排版和节制动效。

**适用场景**：

- 个人工具、写作、任务、设置、轻量 SaaS、偏安静的消费产品。
- 用户明确说“克制”“少一点”“像真实产品而不是展示页”。

**核心约束**：

- 禁止把大圆角、阴影、渐变、玻璃、彩色状态、悬浮卡片当默认。
- 先让字体、留白、层级和细线工作；卡片只在确实需要分层时使用。
- 色彩基底为温和单色，使用一个小面积柔和点缀；图标与图片也要克制。
- 动画只保留反馈性微动效，不做持续抢注意力的视觉秀。

**原始说明**：`.agents/skills/minimalist-ui/SKILL.md`

### 4.13 `industrial-brutalist-ui`

**定位**：工业粗野主义与战术遥测 UI。包含两条主要原型：浅色瑞士工业印刷、深色战术遥测/CRT 终端。

**适用场景**：

- 开发者工具、监控、设备控制、指挥面板、安全研究、游戏化系统工具。
- 品牌本身确实需要冷硬、信息密集、结构外露的表达。

**核心方法**：

- 选浅色工业印刷或深色终端其中一个，不混成“普通深色 SaaS + 绿色字”。
- 使用宏观结构标题、微观等宽数据、强对齐和有意义的分隔线。
- 颜色服务信号和层级，符号/图标像仪表和操作件，而不是装饰贴纸。
- 可加入颗粒、扫描线、印刷纹理，但不影响文字可读性与性能。

**不适用**：疗愈、生活方式、亲子、需要温柔情绪承接的产品。

**原始说明**：`.agents/skills/industrial-brutalist-ui/SKILL.md`

### 4.14 `stitch-design-taste`

**定位**：为 Google Stitch 生成可复用的语义化 `DESIGN.md`。它的交付物是设计系统文档，不是直接代码。

**适用场景**：

- 团队使用 Google Stitch 持续生成多张页面或多屏 App。
- 希望把氛围、色彩、字体、组件、布局、响应式、动效、禁忌写成一个可复用的单一事实来源。

**标准交付结构**：

1. Visual Theme & Atmosphere。
2. Color Palette & Roles，包含名称、十六进制和功能角色。
3. Typography Rules。
4. Component Stylings。
5. Layout Principles。
6. Motion & Interaction。
7. Anti-Patterns。

**关键规则**：最大一个强调色、禁止无目的紫蓝霓虹与纯黑、明确字体角色和禁用字体、定义移动端单列折叠与触控目标、用 `transform/opacity` 做动效、把 AI 模板痕迹写进禁止列表。

**常用请求**：

```text
用 `stitch-design-taste` 为我的游戏接龙后台写 DESIGN.md。
它是高密度运营工具，不要营销感；定义色彩、表格/筛选/表单状态、响应式和明确禁止项。
```

**原始说明**：`.agents/skills/stitch-design-taste/SKILL.md`

## 5. 常用提示词模板

### 5.1 营销网站

```text
用 `design-taste-frontend` + `imagegen-frontend-web` 做实际可用的网站，不做空洞 Landing Page。
产品：...
受众：...
目标动作：...
品牌资产：...
参考：...
视觉方向：...
首屏必须展示：...
不要：紫色 AI 渐变、均分三卡、假数据、无意义动画。
实现前先给 Design Read、三拨盘、区块结构和图像计划。
```

### 5.2 运营后台

```text
用 `frontend-ui-engineering` 改造这个后台页面，并参考 `minimalist-ui` 的克制原则。
用户每天需要反复筛选、比较、保存配置。
保留：...
修复：...
必须有：Loading、Error、Empty、键盘可用、移动端不溢出。
不要把它设计成官网或加夸张滚动动画。
```

### 5.3 参考图还原

```text
用 `image-to-code` 处理附件截图。
模式：严格还原 / 保留结构换品牌 / 只借氛围。
必须还原：...
可替换：...
设备目标：Desktop 1440、Mobile 390。
先分析，再落地；不可凭空把截图改成模板化三卡页面。
```

### 5.4 品牌从零开始

```text
先用 `brandkit` 定义品牌，不要直接写网页。
品牌名：...
行业与受众：...
性格：...
要避开的竞品套路：...
需要：品牌策略、Logo 方向、色彩、字体、影像方向、3x3 品牌板。
```

### 5.5 移动端多屏

```text
用 `imagegen-frontend-mobile`，平台为 iOS-native premium。
核心任务流：...
需要 6 个屏幕：...
要求：明确安全区、导航、空/加载/错误状态、44px 触控目标和跨屏一致性。
```

## 6. 提交前检查清单

### 6.1 视觉

- [ ] 视觉方向来自产品、受众或参考，而不是默认紫色渐变。
- [ ] 字体、色彩、圆角、阴影和间距有统一规则。
- [ ] 有且只有有限的签名元素，其他内容保持克制。
- [ ] 真实图片、人物、产品、场景或数据能被看清，不是氛围遮挡。
- [ ] 没有无意义的三等分卡片、假 KPI、占位名字、Lorem Ipsum、模板套话。

### 6.2 交互与工程

- [ ] 已复用项目已有组件库、图标库、状态管理和设计系统。
- [ ] 所有按钮、链接、表单都能真实执行，不存在 `#` 占位链接。
- [ ] 有 Loading、Empty、Error、Disabled、Hover、Focus、Active 状态。
- [ ] 键盘可用、对比度合格，信息不只靠颜色表达。
- [ ] 在 320、768、1024、1440 宽度下无重叠、无横向溢出。
- [ ] 动画支持 `prefers-reduced-motion`，主要使用 `transform` 与 `opacity`。

### 6.3 选择正确的 Skill

- [ ] 营销页没有被后台工程规范做成平庸表单页。
- [ ] 后台没有被 Awwwards 动效改到无法高频操作。
- [ ] 参考图任务已明确“还原”还是“重设计”。
- [ ] 一次只有一个主审美 Skill。
- [ ] 旧版 v1 仅用于兼容旧页面，而不是新项目默认。

## 7. 原始 Skill 清单

| Skill | 源文件 |
| --- | --- |
| `brandkit` | `.agents/skills/brandkit/SKILL.md` |
| `design-taste-frontend` | `.agents/skills/design-taste-frontend/SKILL.md` |
| `design-taste-frontend-v1` | `.agents/skills/design-taste-frontend-v1/SKILL.md` |
| `gpt-taste` | `.agents/skills/gpt-taste/SKILL.md` |
| `high-end-visual-design` | `.agents/skills/high-end-visual-design/SKILL.md` |
| `image-to-code` | `.agents/skills/image-to-code/SKILL.md` |
| `imagegen-frontend-mobile` | `.agents/skills/imagegen-frontend-mobile/SKILL.md` |
| `imagegen-frontend-web` | `.agents/skills/imagegen-frontend-web/SKILL.md` |
| `industrial-brutalist-ui` | `.agents/skills/industrial-brutalist-ui/SKILL.md` |
| `minimalist-ui` | `.agents/skills/minimalist-ui/SKILL.md` |
| `redesign-existing-projects` | `.agents/skills/redesign-existing-projects/SKILL.md` |
| `stitch-design-taste` | `.agents/skills/stitch-design-taste/SKILL.md` |
| `frontend-design` | `.codex/skills/frontend-design/SKILL.md` |
| `frontend-ui-engineering` | `.codex/skills/frontend-ui-engineering/SKILL.md` |

