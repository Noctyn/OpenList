package feishu

import "github.com/OpenListTeam/OpenList/v4/internal/model"

type BaseResp struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg,omitempty"`
	Message string `json:"message,omitempty"`
}

type ListResp struct {
	BaseResp
	Data ListData `json:"data"`
}

type ListData struct {
	HasMore   bool     `json:"has_more"`
	LastLabel string   `json:"last_label"`
	NodeList  []string `json:"node_list"`
	Entities  struct {
		Nodes map[string]Node `json:"nodes"`
		Users map[string]User `json:"users"`
	} `json:"entities"`
}

type Node struct {
	Token      string `json:"token"`
	NodeToken  string `json:"node_token"`
	ObjToken   string `json:"obj_token"`
	Name       string `json:"name"`
	Type       int    `json:"type"`
	NodeType   int    `json:"node_type"`
	OwnerID    string `json:"owner_id"`
	EditUID    string `json:"edit_uid"`
	CreateTime int64  `json:"create_time"`
	EditTime   int64  `json:"edit_time"`
	URL        string `json:"url"`
	Thumbnail  string `json:"thumbnail"`
	Extra      struct {
		Size    string `json:"size"`
		Subtype string `json:"subtype"`
	} `json:"extra"`
}

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Object struct {
	model.Object
	ObjToken string
	NodeType int
	ObjType  int
	URL      string
	ThumbURL string
}

type CreateFolderResp struct {
	BaseResp
	Data struct {
		Entities struct {
			Nodes map[string]Node `json:"nodes"`
		} `json:"entities"`
		NodeList []string `json:"node_list"`
	} `json:"data"`
}

type RemoveResp struct {
	BaseResp
	Data struct {
		TaskID string `json:"task_id"`
	} `json:"data"`
}

type TaskStatusResp struct {
	BaseResp
	Data struct {
		IsFail   bool `json:"is_fail"`
		IsFinish bool `json:"is_finish"`
	} `json:"data"`
}

type UploadPrepareResp struct {
	BaseResp
	Data UploadPrepareData `json:"data"`
}

type UploadPrepareData struct {
	UploadID        string `json:"upload_id"`
	BlockSize       int64  `json:"block_size"`
	NumBlocks       int    `json:"num_blocks"`
	OptionBlockSize int64  `json:"option_block_size"`
	DedupeSupport   bool   `json:"dedupe_support"`
}

type UploadBlock struct {
	Hash       string `json:"hash"`
	Seq        int    `json:"seq"`
	Size       int64  `json:"size"`
	Checksum   string `json:"checksum"`
	IsUploaded bool   `json:"is_uploaded"`
}

type UploadBlocksResp struct {
	BaseResp
	Data UploadBlocksData `json:"data"`
}

type UploadBlocksData struct {
	NeededUploadBlocks []UploadBlockNeed `json:"needed_upload_blocks"`
}

type UploadBlockNeed struct {
	Seq      int    `json:"seq"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
	Hash     string `json:"hash"`
}

type UploadFinishResp struct {
	BaseResp
	Data UploadFinishData `json:"data"`
}

type UploadFinishData struct {
	FileToken string `json:"file_token"`
	Version   string `json:"version"`
	Extra     struct {
		NodeToken string `json:"node_token"`
	} `json:"extra"`
}
