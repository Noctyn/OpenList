package alidoc

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/cookie"
	"github.com/go-resty/resty/v2"
)

const (
	ChinaSite         = "china"
	InternationalSite = "international"

	chinaBaseURL         = "https://alidocs.dingtalk.com"
	internationalBaseURL = "https://docs.dingtalk.io"
)

func cleanBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
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

func (d *AliDoc) apiBase() string {
	if custom := cleanBaseURL(d.CustomBaseURL); custom != "" {
		return custom
	}

	site := strings.TrimSpace(strings.ToLower(d.Site))
	switch {
	case site == InternationalSite || site == "intl" || site == "global" || site == "io" || strings.Contains(site, "dingtalk.io"):
		return internationalBaseURL
	case strings.HasPrefix(site, "http://") || strings.HasPrefix(site, "https://"):
		if cleaned := cleanBaseURL(site); cleaned != "" {
			return cleaned
		}
		return strings.TrimRight(site, "/")
	default:
		return chinaBaseURL
	}
}

func getCookieValue(cookieStr, key string) string {
	for _, c := range cookie.Parse(cookieStr) {
		if strings.EqualFold(c.Name, key) {
			return c.Value
		}
	}
	return ""
}

func (d *AliDoc) request(ctx context.Context) *resty.Request {
	req := d.client.R().
		SetContext(ctx).
		SetHeader("Cookie", d.Cookie).
		SetHeader("Accept", "application/json, text/plain, */*").
		SetHeader("Referer", d.apiBase()+"/").
		SetHeader("Origin", d.apiBase())
	if xsrf := getCookieValue(d.Cookie, "XSRF-TOKEN"); xsrf != "" {
		req.SetHeader("x-xsrf-token", xsrf)
	}
	if corpID := getCookieValue(d.Cookie, "portal_corp_id"); corpID != "" {
		req.SetHeader("corp-id", corpID)
	}
	return req
}

func msToTime(v int64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(v)
}

func checkResp(resp *resty.Response, result apiResp) error {
	if resp != nil && resp.IsError() {
		if msg := result.ErrMessage(); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("http error: %d", resp.StatusCode())
	}
	if !result.IsSuccess || result.Status != 200 {
		msg := result.ErrMessage()
		if msg == "" {
			msg = "request failed"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func toObj(item dentry) model.Obj {
	return &model.Object{
		ID:       item.DentryUUID,
		Name:     item.Name,
		Size:     item.FileSize,
		Modified: msToTime(item.UpdatedTime),
		Ctime:    msToTime(item.CreatedTime),
		IsFolder: item.DentryType == "folder",
	}
}

func firstDownloadURL(resp downloadResp) (string, error) {
	if len(resp.Data.OSSURLPreSignatureInfo.PreSignURLs) == 0 {
		return "", fmt.Errorf("empty download url")
	}
	return resp.Data.OSSURLPreSignatureInfo.PreSignURLs[0], nil
}

func newClient() *resty.Client {
	client := base.NewRestyClient()
	client.SetHeader("User-Agent", base.UserAgent)
	return client
}

func (d *AliDoc) post(ctx context.Context, path string, body interface{}) error {
	var result apiResp
	resp, err := d.request(ctx).
		SetBody(body).
		SetResult(&result).
		SetError(&result).
		Post(d.apiBase() + path)
	if err != nil {
		return err
	}
	return checkResp(resp, result)
}

func (d *AliDoc) checkCookie(ctx context.Context) error {
	var result apiResp
	resp, err := d.request(ctx).
		SetResult(&result).
		SetError(&result).
		Get(d.apiBase() + "/portal/api/v1/mine/info")
	if err != nil {
		return err
	}
	return checkResp(resp, result)
}

func (d *AliDoc) list(ctx context.Context, dentryUUID string) ([]dentry, error) {
	var result listResp
	resp, err := d.request(ctx).
		SetQueryParam("dentryUuid", dentryUUID).
		SetQueryParam("withParentAncestors", "true").
		SetQueryParam("orderType", "SORT_KEY").
		SetQueryParam("sortType", "desc").
		SetQueryParam("listDentrySource", "2").
		SetQueryParam("pageSize", "1000").
		SetResult(&result).
		SetError(&result).
		Get(d.apiBase() + "/box/api/v2/dentry/list")
	if err != nil {
		return nil, err
	}
	if err := checkResp(resp, result.apiResp); err != nil {
		return nil, err
	}
	return result.Data.Children, nil
}

func (d *AliDoc) download(ctx context.Context, dentryUUID string) (downloadResp, error) {
	var result downloadResp
	resp, err := d.request(ctx).
		SetQueryParam("dentryUuid", dentryUUID).
		SetQueryParam("version", "1").
		SetQueryParam("supportDownloadTypes", "URL_PRE_SIGNATURE,HTTP_TO_CENTER").
		SetQueryParam("downloadType", "URL_PRE_SIGNATURE").
		SetResult(&result).
		SetError(&result).
		Get(d.apiBase() + "/box/api/v2/file/download")
	if err != nil {
		return result, err
	}
	if err := checkResp(resp, result.apiResp); err != nil {
		return result, err
	}
	return result, nil
}
