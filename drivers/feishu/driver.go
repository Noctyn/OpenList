package feishu

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/go-resty/resty/v2"
)

type FeishuBase struct {
	model.Storage
	Addition

	client *resty.Client
}

func (d *FeishuBase) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *FeishuBase) Init(ctx context.Context) error {
	d.Cookie = strings.TrimSpace(d.Cookie)
	d.BaseURL = strings.TrimSpace(d.BaseURL)
	d.RootFolderID = strings.TrimSpace(d.RootFolderID)
	if d.Cookie == "" {
		return fmt.Errorf("cookie is empty")
	}
	d.client = newClient()
	if err := d.checkCookie(ctx); err != nil {
		return err
	}
	return nil
}


func (d *FeishuBase) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	parentToken := dir.GetID()
	if parentToken == "0" {
		parentToken = ""
	}

	nodes, err := d.listAllChildren(ctx, parentToken)
	if err != nil {
		return nil, err
	}

	objs := make([]model.Obj, 0, len(nodes))
	for _, node := range nodes {
		if node.Token == "" && node.NodeToken == "" && node.ObjToken == "" {
			continue
		}
		objs = append(objs, d.toObj(node))
	}
	return objs, nil
}

func (d *FeishuBase) toObj(node Node) model.Obj {
	isFolder := node.Type == 0 || node.Type == 4
	size := parseSize(node.Extra.Size)
	name := strings.TrimSpace(node.Name)
	if name == "" {
		if node.ObjToken != "" {
			name = node.ObjToken
		} else if node.NodeToken != "" {
			name = node.NodeToken
		} else {
			name = node.Token
		}
	}
	id := node.Token
	if id == "" {
		id = node.NodeToken
	}
	if id == "" {
		id = node.ObjToken
	}

	return &Object{
		Object: model.Object{
			ID:       id,
			Name:     name,
			Size:     size,
			Modified: time.Unix(node.EditTime, 0),
			Ctime:    time.Unix(node.CreateTime, 0),
			IsFolder: isFolder,
		},
		ObjToken: node.ObjToken,
		NodeType: node.NodeType,
		ObjType:  node.Type,
		URL:      node.URL,
		ThumbURL: node.Thumbnail,
	}
}

func (d *FeishuBase) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	obj, ok := file.(*Object)
	if !ok {
		return nil, fmt.Errorf("invalid object type")
	}
	if obj.IsDir() {
		return nil, fmt.Errorf("directory link not supported")
	}

	// Thumb/preview
	if (args.Type == "preview" || args.Type == "thumb") && obj.ThumbURL != "" {
		return &model.Link{
			URL: obj.ThumbURL,
			Header: http.Header{
				"Cookie":     []string{d.Cookie},
				"Referer":    []string{d.apiBase() + "/drive/home/"},
				"User-Agent": []string{base.UserAgent},
			},
		}, nil
	}

	// Cloud documents (docx, sheet, mindnote, etc.)
	if obj.ObjType != 12 && obj.URL != "" {
		return &model.Link{
			URL: obj.URL,
			Header: http.Header{
				"Cookie":     []string{d.Cookie},
				"Referer":    []string{d.apiBase() + "/drive/home/"},
				"User-Agent": []string{base.UserAgent},
			},
		}, nil
	}

	// Binary files
	fileToken := obj.ObjToken
	if fileToken == "" {
		fileToken = obj.GetID()
	}
	downloadURL := fmt.Sprintf("%s/space/api/box/stream/download/all/%s/", d.apiBase(), fileToken)
	return &model.Link{
		URL: downloadURL,
		Header: http.Header{
			"Cookie":     []string{d.Cookie},
			"Referer":    []string{d.apiBase() + "/drive/home/"},
			"User-Agent": []string{base.UserAgent},
		},
	}, nil
}

func (d *FeishuBase) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	node, err := d.createFolder(ctx, parentDir.GetID(), dirName)
	if err != nil {
		return nil, err
	}
	return d.toObj(node), nil
}

func (d *FeishuBase) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	obj, ok := srcObj.(*Object)
	if !ok {
		return nil, fmt.Errorf("invalid object type")
	}

	if obj.IsDir() {
		if err := d.renameFolder(ctx, obj.GetID(), newName); err != nil {
			return nil, err
		}
	} else {
		fileToken := obj.ObjToken
		if fileToken == "" {
			fileToken = obj.GetID()
		}
		if err := d.renameFile(ctx, fileToken, newName); err != nil {
			return nil, err
		}
	}
	obj.Name = newName
	return srcObj, nil
}

func (d *FeishuBase) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	if err := d.moveObj(ctx, srcObj.GetID(), dstDir.GetID()); err != nil {
		return nil, err
	}
	return srcObj, nil
}

func (d *FeishuBase) Remove(ctx context.Context, obj model.Obj) error {
	return d.removeObj(ctx, []string{obj.GetID()})
}
