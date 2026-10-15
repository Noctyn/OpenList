package alidoc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

func TestCleanBaseURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"   ", ""},
		{"https://docs.dingtalk.io", "https://docs.dingtalk.io"},
		{"https://docs.dingtalk.io/", "https://docs.dingtalk.io"},
		{"https://docs.dingtalk.io/i/desktop", "https://docs.dingtalk.io"},
		{"docs.dingtalk.io", "https://docs.dingtalk.io"},
		{"docs.dingtalk.io/i/desktop", "https://docs.dingtalk.io"},
		{"http://127.0.0.1:8080/some/path", "http://127.0.0.1:8080"},
		{"https://alidocs.dingtalk.com/i/desktop", "https://alidocs.dingtalk.com"},
	}

	for _, tc := range tests {
		actual := cleanBaseURL(tc.input)
		if actual != tc.expected {
			t.Errorf("cleanBaseURL(%q) = %q, want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestApiBase(t *testing.T) {
	tests := []struct {
		name          string
		site          string
		customBaseURL string
		expected      string
	}{
		{
			name:     "default empty",
			site:     "",
			expected: chinaBaseURL,
		},
		{
			name:     "china site",
			site:     "china",
			expected: chinaBaseURL,
		},
		{
			name:     "china uppercase",
			site:     "CHINA",
			expected: chinaBaseURL,
		},
		{
			name:     "international site",
			site:     "international",
			expected: internationalBaseURL,
		},
		{
			name:     "intl alias",
			site:     "intl",
			expected: internationalBaseURL,
		},
		{
			name:     "global alias",
			site:     "global",
			expected: internationalBaseURL,
		},
		{
			name:     "io alias",
			site:     "io",
			expected: internationalBaseURL,
		},
		{
			name:     "dingtalk.io domain in site",
			site:     "https://docs.dingtalk.io/i/desktop",
			expected: internationalBaseURL,
		},
		{
			name:          "custom base url override",
			site:          "china",
			customBaseURL: "https://custom.dingtalk.com",
			expected:      "https://custom.dingtalk.com",
		},
		{
			name:          "custom base url with subpath",
			site:          "international",
			customBaseURL: "https://mycorp.dingtalk.io/i/desktop",
			expected:      "https://mycorp.dingtalk.io",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := &AliDoc{
				Addition: Addition{
					Site:          tc.site,
					CustomBaseURL: tc.customBaseURL,
				},
			}
			if got := d.apiBase(); got != tc.expected {
				t.Errorf("d.apiBase() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestGetCookieValue(t *testing.T) {
	raw := "XSRF-TOKEN=test-xsrf-token; portal_corp_id=ding123456; other=val"
	if got := getCookieValue(raw, "XSRF-TOKEN"); got != "test-xsrf-token" {
		t.Errorf("getCookieValue(XSRF-TOKEN) = %q, want 'test-xsrf-token'", got)
	}
	if got := getCookieValue(raw, "xsrf-token"); got != "test-xsrf-token" {
		t.Errorf("getCookieValue(xsrf-token) case-insensitive = %q, want 'test-xsrf-token'", got)
	}
	if got := getCookieValue(raw, "portal_corp_id"); got != "ding123456" {
		t.Errorf("getCookieValue(portal_corp_id) = %q, want 'ding123456'", got)
	}
	if got := getCookieValue(raw, "PORTAL_CORP_ID"); got != "ding123456" {
		t.Errorf("getCookieValue(PORTAL_CORP_ID) case-insensitive = %q, want 'ding123456'", got)
	}
	if got := getCookieValue(raw, "not_exist"); got != "" {
		t.Errorf("getCookieValue(not_exist) = %q, want ''", got)
	}
}

func TestMockDriverLifecycleAndOperations(t *testing.T) {
	oldConf := conf.Conf
	conf.Conf = &conf.Config{}
	t.Cleanup(func() { conf.Conf = oldConf })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify required headers
		if r.Header.Get("x-xsrf-token") != "test-xsrf" {
			t.Errorf("missing or invalid x-xsrf-token header: %q", r.Header.Get("x-xsrf-token"))
		}
		if r.Header.Get("corp-id") != "ding-test-corp" {
			t.Errorf("missing or invalid corp-id header: %q", r.Header.Get("corp-id"))
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/portal/api/v1/mine/info":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    200,
				"isSuccess": true,
				"data": map[string]interface{}{
					"name": "Test User",
				},
			})
		case "/box/api/v2/dentry/list":
			dentryUUID := r.URL.Query().Get("dentryUuid")
			if dentryUUID != "root-uuid" {
				t.Errorf("expected dentryUuid root-uuid, got %s", dentryUUID)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    200,
				"isSuccess": true,
				"data": map[string]interface{}{
					"children": []map[string]interface{}{
						{
							"dentryUuid":  "child-1",
							"name":        "test-doc.txt",
							"fileSize":    1024,
							"dentryType":  "file",
							"createdTime": 1700000000000,
							"updatedTime": 1700000000000,
						},
						{
							"dentryUuid":  "folder-1",
							"name":        "TestFolder",
							"dentryType":  "folder",
							"createdTime": 1700000000000,
							"updatedTime": 1700000000000,
						},
					},
				},
			})
		case "/box/api/v2/file/download":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    200,
				"isSuccess": true,
				"data": map[string]interface{}{
					"ossUrlPreSignatureInfo": map[string]interface{}{
						"preSignUrls": []string{"https://oss.example.com/download-url"},
					},
				},
			})
		case "/box/api/v2/dentry/createfolder":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "NewFolder" {
				t.Errorf("unexpected folder name: %v", body["name"])
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    200,
				"isSuccess": true,
			})
		case "/box/api/v2/dentry/rename":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "RenamedDoc" {
				t.Errorf("unexpected rename: %v", body["name"])
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    200,
				"isSuccess": true,
			})
		case "/box/api/v1/dentry/recycle":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["dentryUuid"] != "child-1" {
				t.Errorf("unexpected recycle uuid: %v", body["dentryUuid"])
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    200,
				"isSuccess": true,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	d := &AliDoc{
		Addition: Addition{
			Site:          "international",
			CustomBaseURL: server.URL,
			Cookie:        "XSRF-TOKEN=test-xsrf; portal_corp_id=ding-test-corp",
		},
	}
	d.RootFolderID = "root-uuid"

	if err := d.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Test List
	objs, err := d.List(ctx, &model.Object{ID: "root-uuid"}, model.ListArgs{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objs, got %d", len(objs))
	}
	if objs[0].GetName() != "test-doc.txt" || objs[0].IsDir() {
		t.Errorf("unexpected obj 0: %+v", objs[0])
	}
	if objs[1].GetName() != "TestFolder" || !objs[1].IsDir() {
		t.Errorf("unexpected obj 1: %+v", objs[1])
	}

	// Test Link
	link, err := d.Link(ctx, objs[0], model.LinkArgs{})
	if err != nil {
		t.Fatalf("Link failed: %v", err)
	}
	if link.URL != "https://oss.example.com/download-url" {
		t.Errorf("unexpected link URL: %s", link.URL)
	}
	if link.Header.Get("Referer") != server.URL+"/" {
		t.Errorf("unexpected referer header: %s", link.Header.Get("Referer"))
	}

	// Test MakeDir
	_, err = d.MakeDir(ctx, &model.Object{ID: "root-uuid"}, "NewFolder")
	if err != nil {
		t.Fatalf("MakeDir failed: %v", err)
	}

	// Test Rename
	renamed, err := d.Rename(ctx, &model.Object{ID: "child-1", Name: "old-doc.txt"}, "RenamedDoc")
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
	if renamed.GetName() != "RenamedDoc" {
		t.Errorf("expected renamed name RenamedDoc, got %s", renamed.GetName())
	}

	// Test Remove
	err = d.Remove(ctx, &model.Object{ID: "child-1"})
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Test Drop
	if err := d.Drop(ctx); err != nil {
		t.Fatalf("Drop failed: %v", err)
	}
}

func TestMsToTime(t *testing.T) {
	if !msToTime(0).IsZero() {
		t.Errorf("msToTime(0) should be zero time")
	}
	tm := msToTime(1700000000000)
	if tm.Unix() != 1700000000 {
		t.Errorf("msToTime(1700000000000) = %v", tm)
	}
	_ = time.Second
}
func TestLiveDingTalk(t *testing.T) {
	intlCookie := os.Getenv("DINGTALK_INTL_COOKIE")
	cnCookie := os.Getenv("DINGTALK_CN_COOKIE")
	if intlCookie == "" && cnCookie == "" {
		t.Skip("skipping live DingTalk test: set DINGTALK_INTL_COOKIE or DINGTALK_CN_COOKIE to run")
	}

	oldConf := conf.Conf
	conf.Conf = &conf.Config{}
	t.Cleanup(func() { conf.Conf = oldConf })

	ctx := context.Background()

	if intlCookie != "" {
		rootFolderID := os.Getenv("DINGTALK_INTL_ROOT_ID")
		if rootFolderID == "" {
			rootFolderID = "YGl6Pm2Db8DA62eXRPPMulWxLq0Ee4p7"
		}
		dIntl := &AliDoc{
			Addition: Addition{
				Site:   "international",
				Cookie: intlCookie,
			},
		}
		dIntl.RootFolderID = rootFolderID
		if err := dIntl.Init(ctx); err != nil {
			t.Logf("Live intl cookie may have expired: %v", err)
		} else {
			objs, err := dIntl.List(ctx, &model.Object{ID: dIntl.RootFolderID}, model.ListArgs{})
			if err != nil {
				t.Fatalf("Live intl List failed: %v", err)
			}
			t.Logf("Live intl List successful, returned %d objects", len(objs))
			_ = dIntl.Drop(ctx)
		}
	}

	if cnCookie != "" {
		rootFolderID := os.Getenv("DINGTALK_CN_ROOT_ID")
		if rootFolderID == "" {
			rootFolderID = "2KAYQBnd5ExVEmMrLDKUrVyeZqMmz4Pj"
		}
		dCN := &AliDoc{
			Addition: Addition{
				Site:   "china",
				Cookie: cnCookie,
			},
		}
		dCN.RootFolderID = rootFolderID
		if err := dCN.Init(ctx); err != nil {
			t.Logf("Live cn cookie may have expired: %v", err)
		} else {
			objsCN, err := dCN.List(ctx, &model.Object{ID: dCN.RootFolderID}, model.ListArgs{})
			if err != nil {
				t.Fatalf("Live cn List failed: %v", err)
			}
			t.Logf("Live cn List successful, returned %d objects", len(objsCN))
			_ = dCN.Drop(ctx)
		}
	}
}
