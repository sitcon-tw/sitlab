# SITCON Board Design System

## Direction

SITCON Board 是籌備團隊每天重複使用的工作介面。視覺應安靜、緊湊、可掃描，資訊與操作回饋優先於裝飾。第一 viewport 必須直接識別 `SITCON / 2027` 並顯示可操作的 Board，不使用 marketing hero、gradient、裝飾圖形或永久 sidebar。

`packages/ui/src/styles/tokens.css` 是瀏覽器 token 唯一來源。產品 CSS 使用 `--sb-*` semantic roles；共用 primitives 透過 `--pt-*` aliases 使用同一組值。其他 CSS/TSX 不可加入 raw colors。

## Visual Roles

- 中性 page、lane、surface 與 control 建立工作層級。
- 綠色只用於品牌、主要動作與成功。
- 藍色表示 focus、Inbox 與資訊，黃色表示提醒/review，紅色表示失敗或逾期。
- 卡片只有細邊框與低陰影，lane 是 Board 結構而不是裝飾卡片。
- 圓角不超過 8px；圓形只用於 avatar、count 與 radio。
- 字體不隨 viewport 連續縮放，letter spacing 固定為 0。
- Dark header 使用 `sitcon-tw/2027` source 提供的官方白色 SITCON logo，年度色票沿用該網站，不自行重畫品牌資產。

## Product Layout

Header 高度固定，包含產品識別、成員 Sheet、錯誤時才出現的離線狀態與帳號選單。快速開卡在桌面為單列，手機為 title 加控制列。Board lanes 固定依序為 `Wating`、`Inbox`、`To Do`、`Doing`、`Review`、`Closed`，保持穩定最小寬度；窄螢幕水平捲動，不壓縮到文字與控制重疊。

完整成員目錄只出現在右側 Drawer；桌面為窄 Sheet、手機可佔全寬。不建立永久 sidebar，也不在主 Board 重複完整名單。

## Interaction

- Production initial render 使用 injected bootstrap，不顯示 loading page、skeleton、spinner 或空 Board。
- 背景刷新不替換成 loading state，不改變 layout；健康、pending 與 processing 狀態不顯示，只有離線或 mutation 失敗才顯示技術狀態。
- 開卡與卡片 mutation 立即 optimistic update。失敗保留使用者意圖並顯示 Retry。
- 拖放是滑鼠捷徑；卡片詳細視窗內的狀態 select 提供完整鍵盤操作，卡片表面不重複組別與狀態 controls。
- 卡片詳細視窗包含 title、description、組別、狀態、多人 Assignee、期限、Start time 與 GitLab Issue 連結。
- Assignee picker 支援複選，順序為目前使用者、目前組別、其他組別、未分組；搜尋涵蓋所有 active project members。
- Avatar 固定尺寸，initials 立即顯示；成功載入的圖片原地淡入，失敗不顯示破圖。
- Dialog/Drawer trap focus、Escape 關閉並還原 trigger focus。所有 icon-only controls 有 accessible name 與 tooltip。

## Responsive Review

必要檢查寬度為 320、608、928、1440 pixels。每個寬度都要確認：

1. Header 與 quick-create controls 不重疊。
2. Title、姓名與錯誤訊息不超出容器。
3. Board 可水平捲動且每個 lane/card 維持可讀。
4. 成員 Drawer、Assignee dialog 與 account menu 可完整操作。
5. Focus outline 可見，color 不是狀態的唯一訊號。
