package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/adler32"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/pkg/cookie"
	"github.com/go-resty/resty/v2"
)

var defaultObjTypes = []string{"0", "12", "22", "124"}

func cleanBaseURL(raw string, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}
	return strings.TrimRight(raw, "/")
}

func getCsrfToken(cookieStr string) string {
	cookies := cookie.Parse(cookieStr)
	for _, name := range []string{"_csrf_token", "csrf_token", "swp_csrf_token"} {
		for _, c := range cookies {
			if strings.EqualFold(c.Name, name) && strings.TrimSpace(c.Value) != "" {
				return c.Value
			}
		}
	}
	return ""
}

func newClient() *resty.Client {
	client := base.NewRestyClient()
	client.SetHeader("User-Agent", base.UserAgent)
	return client
}

func parseSize(size string) int64 {
	size = strings.TrimSpace(size)
	if size == "" {
		return 0
	}
	s, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		return 0
	}
	return s
}

func adler32String(data []byte) string {
	sum := adler32.Checksum(data)
	return strconv.FormatUint(uint64(sum), 10)
}

func decodeBaseResp(body []byte, res *resty.Response) error {
	var common BaseResp
	if err := json.Unmarshal(body, &common); err != nil {
		return fmt.Errorf("decode response failed (status: %d): %v", res.StatusCode(), err)
	}
	if common.Code != 0 {
		errMsg := common.Msg
		if errMsg == "" {
			errMsg = common.Message
		}
		return fmt.Errorf("API error (code %d): %s", common.Code, errMsg)
	}
	return nil
}

func (d *FeishuBase) apiBase() string {
	return cleanBaseURL(d.BaseURL, "https://my.feishu.cn")
}

func (d *FeishuBase) request(ctx context.Context, path string, method string, callback base.ReqCallback, resp interface{}) ([]byte, error) {
	req := d.client.R()
	req.SetContext(ctx)
	req.SetHeader("accept", "application/json, text/plain, */*")
	req.SetHeader("origin", d.apiBase())
	req.SetHeader("referer", d.apiBase()+"/drive/home/")
	req.SetHeader("Cookie", d.Cookie)
	if csrf := getCsrfToken(d.Cookie); csrf != "" {
		req.SetHeader("x-csrftoken", csrf)
	}

	if callback != nil {
		callback(req)
	}

	res, err := req.Execute(method, d.apiBase()+path)
	if err != nil {
		return nil, err
	}

	for _, c := range res.Cookies() {
		if strings.EqualFold(c.Name, "_csrf_token") && c.Value != "" {
			d.Cookie = cookie.SetStr(d.Cookie, c.Name, c.Value)
		}
	}

	body := res.Body()
	if resp != nil {
		if err := json.Unmarshal(body, resp); err != nil {
			return body, fmt.Errorf("decode response failed (status: %d): %v", res.StatusCode(), err)
		}
	}
	if err := decodeBaseResp(body, res); err != nil {
		return body, err
	}
	return body, nil
}

func (d *FeishuBase) checkCookie(ctx context.Context) error {
	var resp ListResp
	_, err := d.request(ctx, "/space/api/explorer/recent/list/", http.MethodGet, func(req *resty.Request) {
		req.SetQueryParam("length", "1")
		req.SetQueryParam("rank_type", "0")
	}, &resp)
	return err
}

func (d *FeishuBase) listChildren(ctx context.Context, parentToken string, lastLabel string, length int) (ListData, error) {
	var resp ListResp
	_, err := d.request(ctx, "/space/api/explorer/v3/children/list/", http.MethodGet, func(req *resty.Request) {
		values := url.Values{}
		for _, t := range defaultObjTypes {
			values.Add("obj_type", t)
		}
		values.Set("length", strconv.Itoa(length))
		values.Set("rank", "0")
		values.Set("asc", "0")
		if parentToken != "" {
			values.Set("token", parentToken)
		}
		if lastLabel != "" {
			values.Set("last_label", lastLabel)
		}
		req.SetQueryParamsFromValues(values)
	}, &resp)
	if err != nil {
		return ListData{}, err
	}
	return resp.Data, nil
}

func (d *FeishuBase) listAllChildren(ctx context.Context, parentToken string) ([]Node, error) {
	length := 50
	nodes := make([]Node, 0, length)
	lastLabel := ""
	for range 100 {
		data, err := d.listChildren(ctx, parentToken, lastLabel, length)
		if err != nil {
			return nil, err
		}

		if len(data.NodeList) > 0 {
			for _, token := range data.NodeList {
				if node, ok := data.Entities.Nodes[token]; ok {
					nodes = append(nodes, node)
				}
			}
		} else if len(data.Entities.Nodes) > 0 {
			for token, node := range data.Entities.Nodes {
				if parentToken == "" && (node.Type == 4 || node.Type == 0) && node.Name == "我的空间" {
					continue
				}
				if node.Token == "" {
					node.Token = token
				}
				nodes = append(nodes, node)
			}
		}

		if !data.HasMore || data.LastLabel == "" || data.LastLabel == lastLabel {
			break
		}
		lastLabel = data.LastLabel
	}
	return nodes, nil
}

func (d *FeishuBase) createFolder(ctx context.Context, parentToken, name string) (Node, error) {
	data := url.Values{}
	data.Set("name", name)
	data.Set("source", "0")
	if parentToken != "" {
		data.Set("parent_token", parentToken)
	}

	var resp CreateFolderResp
	_, err := d.request(ctx, "/space/api/explorer/v2/create/folder/", http.MethodPost, func(req *resty.Request) {
		req.SetHeader("Content-Type", "application/x-www-form-urlencoded")
		req.SetBody(data.Encode())
	}, &resp)
	if err != nil {
		return Node{}, err
	}

	for _, token := range resp.Data.NodeList {
		if node, ok := resp.Data.Entities.Nodes[token]; ok {
			return node, nil
		}
	}
	for _, node := range resp.Data.Entities.Nodes {
		return node, nil
	}
	return Node{}, fmt.Errorf("created folder node not found in response")
}

func (d *FeishuBase) renameFolder(ctx context.Context, token, name string) error {
	data := url.Values{}
	data.Set("token", token)
	data.Set("name", name)
	data.Set("type", "0")

	var resp BaseResp
	_, err := d.request(ctx, "/space/api/explorer/v2/rename/", http.MethodPost, func(req *resty.Request) {
		req.SetHeader("Content-Type", "application/x-www-form-urlencoded")
		req.SetBody(data.Encode())
	}, &resp)
	return err
}

func (d *FeishuBase) renameFile(ctx context.Context, fileToken, name string) error {
	var resp BaseResp
	_, err := d.request(ctx, "/space/api/box/file/update_info/", http.MethodPost, func(req *resty.Request) {
		req.SetHeader("Content-Type", "application/json")
		req.SetBody(map[string]interface{}{
			"file_token": fileToken,
			"name":       name,
		})
	}, &resp)
	return err
}

func (d *FeishuBase) moveObj(ctx context.Context, srcToken, destToken string) error {
	data := url.Values{}
	data.Set("src_token", srcToken)
	if destToken != "" {
		data.Set("dest_token", destToken)
	}

	var resp BaseResp
	_, err := d.request(ctx, "/space/api/explorer/v2/move/", http.MethodPost, func(req *resty.Request) {
		req.SetHeader("Content-Type", "application/x-www-form-urlencoded")
		req.SetBody(data.Encode())
	}, &resp)
	return err
}

func (d *FeishuBase) removeObj(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	var resp RemoveResp
	_, err := d.request(ctx, "/space/api/explorer/v3/remove/", http.MethodPost, func(req *resty.Request) {
		req.SetHeader("Content-Type", "application/json")
		req.SetBody(map[string]interface{}{
			"tokens": tokens,
			"apply":  1,
		})
	}, &resp)
	if err != nil {
		return err
	}
	return d.waitTask(ctx, resp.Data.TaskID)
}

func (d *FeishuBase) waitTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return nil
	}
	for range 30 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}

		var resp TaskStatusResp
		_, err := d.request(ctx, "/space/api/explorer/v2/task/", http.MethodGet, func(req *resty.Request) {
			req.SetQueryParam("task_id", taskID)
		}, &resp)
		if err != nil {
			return err
		}
		if resp.Data.IsFail {
			return fmt.Errorf("task %s failed", taskID)
		}
		if resp.Data.IsFinish {
			return nil
		}
	}
	return nil
}
