package feishu

import (
	"context"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	driver.RootID
	BaseURL string `json:"base_url" default:"https://my.feishu.cn" help:"站点域名：飞书默认 https://my.feishu.cn；LarkSuite 用户请填写租户域名如 https://xxxx.larksuite.com，支持直接填入首页 URL"`
	Cookie  string `json:"cookie" type:"text" required:"true" help:"网页端 Cookie（从浏览器开发者工具复制）"`
}

var feishuConfig = driver.Config{
	Name:        "Feishu",
	LocalSort:   true,
	DefaultRoot: "",
	PreferProxy: true,
}

var larkConfig = driver.Config{
	Name:        "LarkSuite",
	LocalSort:   true,
	DefaultRoot: "",
	PreferProxy: true,
}

type Feishu struct {
	FeishuBase
}

func (d *Feishu) Config() driver.Config {
	return feishuConfig
}

func (d *Feishu) Drop(ctx context.Context) error {
	d.client = nil
	if d.GetStorage().ID != 0 {
		op.MustSaveDriverStorage(d)
	}
	return nil
}
type LarkSuite struct {
	FeishuBase
}

func (d *LarkSuite) Config() driver.Config {
	return larkConfig
}

func (d *LarkSuite) Drop(ctx context.Context) error {
	d.client = nil
	if d.GetStorage().ID != 0 {
		op.MustSaveDriverStorage(d)
	}
	return nil
}

var (
	_ driver.Driver       = (*Feishu)(nil)
	_ driver.MkdirResult  = (*Feishu)(nil)
	_ driver.MoveResult   = (*Feishu)(nil)
	_ driver.RenameResult = (*Feishu)(nil)
	_ driver.Remove       = (*Feishu)(nil)
	_ driver.Put          = (*Feishu)(nil)

	_ driver.Driver       = (*LarkSuite)(nil)
	_ driver.MkdirResult  = (*LarkSuite)(nil)
	_ driver.MoveResult   = (*LarkSuite)(nil)
	_ driver.RenameResult = (*LarkSuite)(nil)
	_ driver.Remove       = (*LarkSuite)(nil)
	_ driver.Put          = (*LarkSuite)(nil)
)

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &Feishu{
			FeishuBase: FeishuBase{
				Addition: Addition{
					BaseURL: "https://my.feishu.cn",
				},
			},
		}
	})
	op.RegisterDriver(func() driver.Driver {
		return &LarkSuite{
			FeishuBase: FeishuBase{
				Addition: Addition{
					BaseURL: "https://www.larksuite.com",
				},
			},
		}
	})
}
