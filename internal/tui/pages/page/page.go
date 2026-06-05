package page

// Page 是 root Bubble Tea model 的页面路由状态。
// 用显式类型替代裸字符串，避免 child model / 子包之间页面名散落各处。
type Page string

const (
	None    Page = ""
	Welcome Page = "welcome"
	Chat    Page = "chat"
	Config  Page = "config"
	Help    Page = "help"
)
