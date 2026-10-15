package alidoc

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	driver.RootID
	Site          string `json:"site" type:"select" options:"china,international" default:"china" help:"站点版本：china (国内版 alidocs.dingtalk.com), international (国际版 docs.dingtalk.io)"`
	CustomBaseURL string `json:"custom_base_url" help:"自定义接口地址 (例如 https://docs.dingtalk.io)，留空则根据站点版本自动配置"`
	Cookie        string `json:"cookie" type:"text" required:"true" help:"钉钉文档网页 Cookie"`
}

var config = driver.Config{
	Name:        "AliDoc",
	LocalSort:   true,
	DefaultRoot: "",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &AliDoc{
			Addition: Addition{
				Site: ChinaSite,
			},
		}
	})
}
