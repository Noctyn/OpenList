package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

func TestCleanBaseURL(t *testing.T) {
	tests := []struct {
		input    string
		fallback string
		expected string
	}{
		{"", "https://my.feishu.cn", "https://my.feishu.cn"},
		{"   ", "https://www.larksuite.com", "https://www.larksuite.com"},
		{"https://my.feishu.cn/drive/home/", "https://my.feishu.cn", "https://my.feishu.cn"},
		{"https://my.feishu.cn", "https://my.feishu.cn", "https://my.feishu.cn"},
		{"https://njpxamr4capj.jp.larksuite.com/drive/home/", "https://www.larksuite.com", "https://njpxamr4capj.jp.larksuite.com"},
		{"njpxamr4capj.jp.larksuite.com", "https://www.larksuite.com", "https://njpxamr4capj.jp.larksuite.com"},
		{"http://127.0.0.1:8080/drive/home/", "", "http://127.0.0.1:8080"},
	}

	for _, tc := range tests {
		actual := cleanBaseURL(tc.input, tc.fallback)
		if actual != tc.expected {
			t.Errorf("cleanBaseURL(%q, %q) = %q, want %q", tc.input, tc.fallback, actual, tc.expected)
		}
	}
}

func TestGetCsrfToken(t *testing.T) {
	tests := []struct {
		cookie string
		expect string
	}{
		{"_csrf_token=csrf-token-123; session=abc", "csrf-token-123"},
		{"_CSRF_TOKEN=csrf-upper-456; session=abc", "csrf-upper-456"},
		{"csrf_token=csrf-alt-789; session=abc", "csrf-alt-789"},
		{"swp_csrf_token=swp-token-111; session=abc", "swp-token-111"},
		{"other=123; session=abc", ""},
	}

	for _, tc := range tests {
		actual := getCsrfToken(tc.cookie)
		if actual != tc.expect {
			t.Errorf("getCsrfToken(%q) = %q, want %q", tc.cookie, actual, tc.expect)
		}
	}
}

func TestMockFeishuLifecycle(t *testing.T) {
	oldConf := conf.Conf
	conf.Conf = &conf.Config{}
	t.Cleanup(func() { conf.Conf = oldConf })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-csrftoken") != "test-csrf-token" {
			t.Errorf("missing or invalid x-csrftoken: %q", r.Header.Get("x-csrftoken"))
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.URL.Path {
		case "/space/api/explorer/recent/list/":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"msg":  "Success",
				"data": map[string]interface{}{
					"total": 0,
				},
			})
		case "/space/api/explorer/v3/children/list/":
			token := r.URL.Query().Get("token")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"msg":  "Success",
				"data": map[string]interface{}{
					"has_more":   false,
					"last_label": "",
					"node_list":  []string{"node-file-1", "node-folder-1"},
					"entities": map[string]interface{}{
						"nodes": map[string]interface{}{
							"node-file-1": map[string]interface{}{
								"token":      "node-file-1",
								"obj_token":  "obj-file-1",
								"name":       "document.pdf",
								"type":       12,
								"node_type":  0,
								"edit_time":  1700000000,
								"thumbnail":  "https://cover.example.com/thumb",
								"extra": map[string]interface{}{
									"size": "2048",
								},
							},
							"node-folder-1": map[string]interface{}{
								"token":      "node-folder-1",
								"obj_token":  "obj-folder-1",
								"name":       "FolderA",
								"type":       0,
								"node_type":  0,
								"edit_time":  1700000000,
								"extra": map[string]interface{}{
									"size": "0",
								},
							},
						},
					},
				},
			})
			_ = token
		case "/space/api/explorer/v2/create/folder/":
			_ = r.ParseForm()
			name := r.Form.Get("name")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"msg":  "Success",
				"data": map[string]interface{}{
					"node_list": []string{"new-folder-token"},
					"entities": map[string]interface{}{
						"nodes": map[string]interface{}{
							"new-folder-token": map[string]interface{}{
								"token":     "new-folder-token",
								"obj_token": "new-folder-token",
								"name":      name,
								"type":      0,
								"edit_time": 1700000000,
							},
						},
					},
				},
			})
		case "/space/api/explorer/v2/rename/":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"msg":  "Success",
				"data": map[string]interface{}{},
			})
		case "/space/api/box/file/update_info/":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"msg":  "Success",
			})
		case "/space/api/explorer/v2/move/":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"msg":  "Success",
				"data": map[string]interface{}{
					"task_id": "",
				},
			})
		case "/space/api/explorer/v3/remove/":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"msg":  "Success",
				"data": map[string]interface{}{
					"task_id": "mock-task-1",
				},
			})
		case "/space/api/explorer/v2/task/":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"msg":  "Success",
				"data": map[string]interface{}{
					"is_fail":   false,
					"is_finish": true,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx := context.Background()

	// 1. Feishu driver test
	dFeishu := &Feishu{
		FeishuBase: FeishuBase{
			Addition: Addition{
				BaseURL: server.URL,
				Cookie:  "_csrf_token=test-csrf-token; session=test-session-123",
			},
		},
	}
	if dFeishu.Config().Name != "Feishu" {
		t.Fatalf("unexpected driver name: %s", dFeishu.Config().Name)
	}
	if err := dFeishu.Init(ctx); err != nil {
		t.Fatalf("Feishu Init failed: %v", err)
	}

	objs, err := dFeishu.List(ctx, &model.Object{ID: ""}, model.ListArgs{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objs, got %d", len(objs))
	}
	if objs[0].GetName() != "document.pdf" || objs[0].IsDir() {
		t.Errorf("unexpected obj[0]: %+v", objs[0])
	}
	if objs[1].GetName() != "FolderA" || !objs[1].IsDir() {
		t.Errorf("unexpected obj[1]: %+v", objs[1])
	}

	// Link test
	link, err := dFeishu.Link(ctx, objs[0], model.LinkArgs{})
	if err != nil {
		t.Fatalf("Link failed: %v", err)
	}
	expectedDownload := server.URL + "/space/api/box/stream/download/all/obj-file-1/"
	if link.URL != expectedDownload {
		t.Errorf("link URL = %q, want %q", link.URL, expectedDownload)
	}

	// Thumb link test
	thumbLink, err := dFeishu.Link(ctx, objs[0], model.LinkArgs{Type: "thumb"})
	if err != nil {
		t.Fatalf("Thumb link failed: %v", err)
	}
	if thumbLink.URL != "https://cover.example.com/thumb" {
		t.Errorf("thumb link URL = %q, want 'https://cover.example.com/thumb'", thumbLink.URL)
	}

	// MakeDir test
	newDir, err := dFeishu.MakeDir(ctx, &model.Object{ID: ""}, "NewFolder")
	if err != nil {
		t.Fatalf("MakeDir failed: %v", err)
	}
	if newDir.GetName() != "NewFolder" || !newDir.IsDir() {
		t.Errorf("unexpected created dir: %+v", newDir)
	}

	// Rename folder
	renamedDir, err := dFeishu.Rename(ctx, newDir, "RenamedFolder")
	if err != nil {
		t.Fatalf("Rename folder failed: %v", err)
	}
	if renamedDir.GetName() != "RenamedFolder" {
		t.Errorf("renamed folder name = %s", renamedDir.GetName())
	}

	// Rename file
	renamedFile, err := dFeishu.Rename(ctx, objs[0], "document-renamed.pdf")
	if err != nil {
		t.Fatalf("Rename file failed: %v", err)
	}
	if renamedFile.GetName() != "document-renamed.pdf" {
		t.Errorf("renamed file name = %s", renamedFile.GetName())
	}

	// Move test
	moved, err := dFeishu.Move(ctx, renamedFile, newDir)
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}
	if moved == nil {
		t.Fatalf("expected moved obj")
	}

	// Remove test
	if err := dFeishu.Remove(ctx, renamedFile); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if err := dFeishu.Drop(ctx); err != nil {
		t.Fatalf("Drop failed: %v", err)
	}

	// 2. LarkSuite driver test
	dLark := &LarkSuite{
		FeishuBase: FeishuBase{
			Addition: Addition{
				BaseURL: server.URL,
				Cookie:  "_csrf_token=test-csrf-token; session=test-lark-session",
			},
		},
	}
	if dLark.Config().Name != "LarkSuite" {
		t.Fatalf("unexpected driver name: %s", dLark.Config().Name)
	}
	if err := dLark.Init(ctx); err != nil {
		t.Fatalf("Lark Init failed: %v", err)
	}
	if err := dLark.Drop(ctx); err != nil {
		t.Fatalf("Lark Drop failed: %v", err)
	}
}

func TestLiveFeishuAndLark(t *testing.T) {
	feishuCookie := os.Getenv("FEISHU_COOKIE")
	larkCookie := os.Getenv("LARK_COOKIE")
	if feishuCookie == "" && larkCookie == "" {
		t.Skip("skipping live Feishu & Lark test; set FEISHU_COOKIE or LARK_COOKIE to run")
	}

	oldConf := conf.Conf
	conf.Conf = &conf.Config{}
	t.Cleanup(func() { conf.Conf = oldConf })

	ctx := context.Background()

	if feishuCookie != "" {
		feishuURL := os.Getenv("FEISHU_URL")
		if feishuURL == "" {
			feishuURL = "https://my.feishu.cn"
		}
		dFeishu := &Feishu{
			FeishuBase: FeishuBase{
				Addition: Addition{
					BaseURL: feishuURL,
					Cookie:  feishuCookie,
				},
			},
		}
		if err := dFeishu.Init(ctx); err != nil {
			t.Logf("Live Feishu Init error: %v", err)
		} else {
			objs, err := dFeishu.List(ctx, &model.Object{ID: ""}, model.ListArgs{})
			if err != nil {
				t.Fatalf("Live Feishu List failed: %v", err)
			}
			t.Logf("Live Feishu List succeeded: found %d objects in root", len(objs))
			_ = dFeishu.Drop(ctx)
		}
	}

	if larkCookie != "" {
		larkURL := os.Getenv("LARK_URL")
		if larkURL == "" {
			larkURL = "https://njpxamr4capj.jp.larksuite.com"
		}
		dLark := &LarkSuite{
			FeishuBase: FeishuBase{
				Addition: Addition{
					BaseURL: larkURL,
					Cookie:  larkCookie,
				},
			},
		}
		if err := dLark.Init(ctx); err != nil {
			t.Logf("Live Lark Init error: %v", err)
		} else {
			objs, err := dLark.List(ctx, &model.Object{ID: ""}, model.ListArgs{})
			if err != nil {
				t.Fatalf("Live Lark List failed: %v", err)
			}
			t.Logf("Live Lark List succeeded: found %d objects in root", len(objs))
			_ = dLark.Drop(ctx)
		}
	}
}

func TestParseSize(t *testing.T) {
	if parseSize("") != 0 {
		t.Errorf("parseSize('') should be 0")
	}
	if parseSize("1024") != 1024 {
		t.Errorf("parseSize('1024') = %d, want 1024", parseSize("1024"))
	}
	if parseSize("abc") != 0 {
		t.Errorf("parseSize('abc') should be 0")
	}
}
