package feishu

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/go-resty/resty/v2"
)

func (d *FeishuBase) prepareUpload(ctx context.Context, name string, size int64, mountNodeToken string) (UploadPrepareData, error) {
	var resp UploadPrepareResp
	body := map[string]interface{}{
		"mount_point":      "explorer",
		"mount_node_token": mountNodeToken,
		"name":             name,
		"size":             size,
		"size_checker":     true,
	}
	_, err := d.request(ctx, "/space/api/box/upload/prepare/", http.MethodPost, func(req *resty.Request) {
		req.SetHeader("Content-Type", "application/json")
		req.SetBody(body)
	}, &resp)
	if err != nil {
		return UploadPrepareData{}, err
	}
	return resp.Data, nil
}

func (d *FeishuBase) uploadBlocks(ctx context.Context, uploadID string, blocks []UploadBlock, mountPoint string) (UploadBlocksData, error) {
	if uploadID == "" {
		return UploadBlocksData{}, fmt.Errorf("upload blocks missing upload_id")
	}
	if mountPoint == "" {
		mountPoint = "explorer"
	}
	var resp UploadBlocksResp
	_, err := d.request(ctx, "/space/api/box/upload/blocks/", http.MethodPost, func(req *resty.Request) {
		req.SetHeader("Content-Type", "application/json")
		req.SetBody(map[string]interface{}{
			"upload_id":   uploadID,
			"blocks":      blocks,
			"mount_point": mountPoint,
		})
	}, &resp)
	if err != nil {
		return UploadBlocksData{}, err
	}
	return resp.Data, nil
}

func (d *FeishuBase) uploadBlockV3(ctx context.Context, uploadID string, block UploadBlockNeed, data []byte) error {
	if uploadID == "" {
		return fmt.Errorf("upload block missing upload_id")
	}

	qParams := url.Values{}
	qParams.Set("upload_id", uploadID)
	qParams.Set("seq", strconv.Itoa(block.Seq))
	qParams.Set("size", strconv.FormatInt(int64(len(data)), 10))
	qParams.Set("checksum", block.Checksum)
	qParams.Set("mount_point", "explorer")

	path := "/space/api/box/stream/upload/v3/block/?" + qParams.Encode()
	_, err := d.request(ctx, path, http.MethodPost, func(req *resty.Request) {
		req.SetHeader("x-block-seq", strconv.Itoa(block.Seq))
		req.SetHeader("x-block-checksum", block.Checksum)
		req.SetMultipartFormData(map[string]string{
			"upload_id": uploadID,
			"size":      strconv.FormatInt(int64(len(data)), 10),
		})
		req.SetMultipartField("file", "blob", "application/octet-stream", bytes.NewReader(data))
	}, nil)
	return err
}

func (d *FeishuBase) finishUpload(ctx context.Context, uploadID string, numBlocks int, mountPoint string) (UploadFinishData, error) {
	if uploadID == "" {
		return UploadFinishData{}, fmt.Errorf("finish upload missing upload_id")
	}
	if mountPoint == "" {
		mountPoint = "explorer"
	}

	var resp UploadFinishResp
	_, err := d.request(ctx, "/space/api/box/upload/finish/", http.MethodPost, func(req *resty.Request) {
		req.SetHeader("Content-Type", "application/json")
		req.SetBody(map[string]interface{}{
			"upload_id":   uploadID,
			"num_blocks":  numBlocks,
			"mount_point": mountPoint,
		})
	}, &resp)
	if err != nil {
		return UploadFinishData{}, err
	}
	return resp.Data, nil
}

func (d *FeishuBase) Put(ctx context.Context, dstDir model.Obj, file model.FileStreamer, up driver.UpdateProgress) error {
	totalSize := file.GetSize()
	mountToken := dstDir.GetID()

	uploadPrep, err := d.prepareUpload(ctx, file.GetName(), totalSize, mountToken)
	if err != nil {
		return err
	}

	// 0-byte file: finish immediately with num_blocks = 0
	if totalSize == 0 {
		_, err = d.finishUpload(ctx, uploadPrep.UploadID, 0, "explorer")
		if err != nil {
			return err
		}
		if up != nil {
			up(100)
		}
		return nil
	}

	tmpFile, err := file.CacheFullAndWriter(&up, nil)
	if err != nil {
		return err
	}
	if c, ok := tmpFile.(io.Closer); ok {
		defer c.Close()
	}

	blockSize := uploadPrep.BlockSize
	if blockSize <= 0 {
		blockSize = 4 * 1024 * 1024
	}

	numBlocks := int((totalSize + blockSize - 1) / blockSize)
	blocks := make([]UploadBlock, 0, numBlocks)

	for seq := range numBlocks {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		offset := int64(seq) * blockSize
		chunkSize := blockSize
		if remain := totalSize - offset; remain < chunkSize {
			chunkSize = remain
		}

		buf := make([]byte, chunkSize)
		n, err := tmpFile.ReadAt(buf, offset)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return err
		}
		buf = buf[:n]

		sum := sha256.Sum256(buf)
		hash := base64.StdEncoding.EncodeToString(sum[:])
		checksum := adler32String(buf)

		block := UploadBlock{
			Hash:       hash,
			Seq:        seq,
			Size:       int64(n),
			Checksum:   checksum,
			IsUploaded: true,
		}
		blocks = append(blocks, block)
	}

	needed, err := d.uploadBlocks(ctx, uploadPrep.UploadID, blocks, "explorer")
	if err != nil {
		return err
	}

	var uploadedBytes int64
	for _, need := range needed.NeededUploadBlocks {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		offset := int64(need.Seq) * blockSize
		chunkSize := blockSize
		if remain := totalSize - offset; remain < chunkSize {
			chunkSize = remain
		}
		buf := make([]byte, chunkSize)
		n, err := tmpFile.ReadAt(buf, offset)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return err
		}
		buf = buf[:n]

		if err := d.uploadBlockV3(ctx, uploadPrep.UploadID, need, buf); err != nil {
			return err
		}
		uploadedBytes += int64(len(buf))
		if up != nil && totalSize > 0 {
			up(float64(uploadedBytes) / float64(totalSize) * 100)
		}
	}

	_, err = d.finishUpload(ctx, uploadPrep.UploadID, numBlocks, "explorer")
	if err != nil {
		return err
	}
	if up != nil {
		up(100)
	}
	return nil
}
