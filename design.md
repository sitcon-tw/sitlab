# SITCON Board Design System

## Direction

SITCON Board 是籌備團隊每天重複使用的工作介面。視覺應安靜、緊湊、可掃描，資訊與操作回饋優先於裝飾。第一 viewport 必須直接識別 `SITCON / 2027` 並顯示可操作的 Board，不使用 marketing hero、gradient、裝飾圖形或永久 sidebar。

SITCON Board 採用 **Material Design 3**。`packages/ui/src/styles/` 是瀏覽器 token 與 primitive 樣式的唯一來源，分三層：`--md-ref-palette-*`（由品牌綠種子產生的色調階，執行 `pnpm --filter @project-template/ui generate:tokens` 重新產生）、`--md-sys-*`（產品 CSS 消費的系統 token）、以及少數保留的產品角色。`tokens.css` 是唯一可以出現具體色值的樣式檔；`md3.css` 與所有 feature CSS 只能用 `var()` 與 `color-mix()`。

## Visual Roles

- 表面層級使用 MD3 的 `surface-container-*` 階梯。**Board 的 page / lane / card 三個 surface 是產品角色**，明暗兩色刻意對應到不同的 `--md-sys-*` role：亮色要卡片最亮（`container-lowest`），暗色要卡片以亮度表示抬升（`container-high`），沒有單一 role 能同時滿足。MD3 本來就預期產品 surface 依 scheme 分別對應。
- 綠色是 MD3 的 `primary` 種子色。`secondary`（focus、已選 chip）與 `tertiary`（資訊、Inbox）是同一顆種子推導出的色調同伴，不是另外挑的顏色。
- focus 用 `secondary`，資訊用 `tertiary`，提醒用自訂的 `warning` 色組（MD3 沒有 warning 角色，SITCON 需要），失敗與逾期用 `error`。
- 卡片是 MD3 elevated card：靜止 level1、hover level2、拖曳中 level3 加 16% dragged state layer，拖曳預覽 level4。lane 是 grouping container，不是裝飾卡片。
- 圓角使用 MD3 shape scale：0 / 4 / 8 / 12 / 16 / 28 / full。卡片 12dp、lane 上緣 16dp、dialog 28dp。按鈕、chip、icon button、segmented button 與 tab 指示器使用 `full`。
- 字體不隨 viewport 連續縮放；letter spacing 依 MD3 type scale 的 per-role tracking，不再固定為 0。
- Dark header 使用 `sitcon-tw/2027` source 提供的官方白色 SITCON logo，年度色票沿用該網站，不自行重畫品牌資產。

### Material Design 3 conformance

- **State layer** 是該表面自身內容色的疊加：hover 8%、focus 10%、pressed 10%、dragged 16%。實作用 `background-color: currentColor` 的 `::before`，所以 filled button 的層是 `on-primary`、outlined 的是 `primary`，不需要 per-variant token，也不需要被政策禁止的 `rgba()`。
- **Ripple** 出現在每一個可操作表面，節點由 React 擁有（`useRipple`），不做 DOM 手術。
- **Focus indicator** 是 3dp `secondary` outline、2dp offset，取代舊的 focus ring。輸入框額外保留這個指示器而不是移除瀏覽器預設 outline。
- **觸控目標** 視覺容器 40dp、觸控區 48dp。48dp 用不影響排版的 `::after` 覆蓋層提供 —— 直接放大容器會讓看板在 320px 超出 viewport。
- **Motion** 使用 MD3 duration 與 easing token。`prefers-reduced-motion` 下移除 ripple 與 transition，但保留 state layer 並補上 pressed 層。
- **Type scale 對應**：卡片標題 `title-medium`、lane 標題 `title-small`、卡片內文 `body-medium`、按鈕與 chip `label-large`、dialog 標題 `headline-small`、app bar `title-large`。

### 明列的 MD3 偏離

以下是刻意不照 MD3 的地方，每一項都有產品理由：

1. **App bar 在明暗兩色都維持 SITCON 墨色表面**，而不是 `surface`。品牌識別需求。
2. **Snackbar region 會堆疊**，MD3 一次只顯示一個。看板需要同時浮現多個失敗。
3. **Ripple 是單階段 500ms 擴散淡出**，而非 MD3 的擴散 + 放開淡出兩階段。
4. **保留原生 `<select>`**，以 MD3 outlined select 的外觀呈現。換掉會失去行動裝置原生選單，MD3 的視覺規範在原生元素上做得到。
5. **保留 Inter 而不引入 Roboto**。tracking 差異在次像素等級，為中文產品加一套 Latin webfont 是負收益。
6. **看板內的緊湊控制項不套用 56dp 浮動標籤欄位**。MD3 沒有要求每個輸入都是 text field，密集資料表面本來就使用緊湊控制項；強制套用會讓卡片高度暴增並弄壞 320px 容納測試。
7. **保留第三階文字色 `--sb-text-subtle`**（`on-surface-variant` 與 `outline` 的混色）。MD3 沒有第三階文字色，而 `outline` 單獨用在文字上只有 4.25:1，低於 AA。

## Product Layout

Header 高度固定，包含產品識別、成員 Sheet、錯誤時才出現的離線狀態與帳號選單。快速開卡在桌面為單列，手機為 title 加控制列，並以 segmented mode 切換單組或所有組長；更多 icon 開啟 Status、Description 與可搜尋的一般 Labels 選擇，Status 預設為 `Inbox`，新卡固定出現在目標欄位最上方。Board 上方的緊湊控制列先提供欄內日期排序，再提供單一組別與依組別複選的成員篩選。Board lanes 固定依序為 `Waiting`、`Inbox`、`To do`、`Doing`、`Review`、`Done`，保持穩定最小寬度；窄螢幕水平捲動，不壓縮到文字與控制重疊。

完整成員目錄只出現在右側 Drawer；桌面為窄 Sheet、手機可佔全寬。不建立永久 sidebar，也不在主 Board 重複完整名單。看板篩選列另提供可搜尋、可複選的全部 GitLab Labels；複選 Labels 採全部符合。

## Interaction

- Production initial render 使用 injected bootstrap，不顯示 loading page、skeleton、spinner 或空 Board。
- 背景刷新不替換成 loading state，不改變 layout；健康的背景 pending 與 processing 狀態不顯示，只有離線或 mutation 失敗才顯示技術狀態。使用者剛編輯的欄位例外：該欄位就地顯示 saving 指示並收斂成短暫的 saved 標記，Drawer 內只用單一 aria-live region 播報，不搶 focus、不 disable 控制項。
- 開卡與卡片 mutation 立即 optimistic update。失敗保留使用者意圖並顯示 Retry。
- 手動排序時可由 grip 使用滑鼠或觸控在同欄與跨欄精確拖放；有篩選時以可見卡片為插入錨點並保留隱藏卡片相對順序。上下移按鈕與卡片右側 detail Drawer 的狀態 select 提供完整鍵盤操作，卡片表面不重複組別與狀態 controls。
- 卡片 detail Drawer 包含前後卡片切換、title、支援 GFM 預覽的 Markdown description、組別、原生 lifecycle status、多人 Assignee、GitLab Start/Due dates、GitLab Labels、typed Quick Actions、Comments 與 Issue 連結。Label 以帶色彩 swatch 的 MD3 input chips 呈現；既有 project labels 可搜尋新增，唯一 Team Label 只能由另一個 Team Label 取代，status 只由獨立控制變更。Comment thread 依時間顯示使用者留言與 GitLab system notes，composer 固定在 thread 下方，送出失敗保留 draft。
- Assignee picker 與成員篩選支援依組別複選；組別標題 checkbox 可切換該組目前可見成員，搜尋時不影響隱藏結果。Assignee picker 以目前組別優先，其他組別依目錄順序，未分組置底；跨組成員出現在每個所屬組別。
- Sort by 在每個 lane 內提供手動、Due、Start 與 Updated time 正反向排序；空日期固定置底，清除篩選不重設排序。
- 組別、負責人、Labels 與 Sort by 以 query string 保存；變更取代目前網址、不新增瀏覽歷史，分享與重新載入會還原相同視圖。
- Quick Create 更多選項使用可取消的 draft；套用後由主列建立按鈕送出，建卡後清空 Description 與 Labels 並保留 Status。所有組長模式將相同設定套用至每張卡片。
- Avatar 固定尺寸，initials 立即顯示；成功載入的圖片原地淡入，失敗不顯示破圖。
- Dialog/Drawer trap focus、Escape 關閉並還原 trigger focus。所有 icon-only controls 有 accessible name 與 tooltip。

## Responsive Review

必要檢查寬度為 320、608、928、1440 pixels。每個寬度都要確認：

1. Header 與 quick-create controls 不重疊。
2. Title、姓名與錯誤訊息不超出容器。
3. Board 可水平捲動且每個 lane/card 維持可讀。
4. 成員 Drawer、Assignee dialog、卡片 Tags/Comments 與 account menu 可完整操作。
5. Focus outline 可見，color 不是狀態的唯一訊號。
6. State layer 與 ripple 在每個可操作表面都看得到；開啟 `prefers-reduced-motion` 時 ripple 消失、pressed state layer 仍在。
