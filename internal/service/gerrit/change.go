package gerrit

import (
	"context"
	"fmt"
	"strconv"

	"github.com/oekit/logx"
)

// GetChange 获取 Gerrit change 详情。
// changeID 可以是数字编号（"12345"）或 Change-Id（"I1234abcd"）。
func (c *Client) GetChange(ctx context.Context, changeID string) (map[string]interface{}, error) {
	change, _, err := c.raw.Changes.GetChange(ctx, changeID, nil)
	if err != nil {
		return nil, fmt.Errorf("gerrit: get change %s: %w", changeID, err)
	}
	return map[string]interface{}{
		"number":           change.Number,
		"project":          change.Project,
		"branch":           change.Branch,
		"status":           change.Status,
		"current_revision": change.CurrentRevision,
		"subject":          change.Subject,
	}, nil
}

// GetHeadSHA 获取 change 当前最新 patchset 的 commit SHA。
// 用于 stale check：比对 webhook 传来的 headSHA 与 Gerrit 服务端的当前值。
func (c *Client) GetHeadSHA(ctx context.Context, changeNumber string) (string, error) {
	change, _, err := c.raw.Changes.GetChange(ctx, changeNumber, nil)
	if err != nil {
		return "", fmt.Errorf("gerrit: get change %s for head SHA: %w", changeNumber, err)
	}

	if change.CurrentRevision != "" {
		return change.CurrentRevision, nil
	}

	// 回退：从 Revisions map 中取最新一个
	if change.Revisions != nil {
		for sha := range change.Revisions {
			return sha, nil
		}
	}

	return "", fmt.Errorf("gerrit: change %s has no revision info", changeNumber)
}

// GetCurrentPatchsetRef 获取 change 当前最新 patchset 的 git ref。
//
// Gerrit change ref 格式: refs/changes/XX/NNNNN/P
//   - XX = changeNumber % 100（两位后缀）
//   - NNNNN = change 数字编号
//   - P = patchset 编号
//
// 返回 ref 和 patchset 编号。
func (c *Client) GetCurrentPatchsetRef(ctx context.Context, changeNumber string) (ref string, patchset int, err error) {
	change, _, err := c.raw.Changes.GetChange(ctx, changeNumber, nil)
	if err != nil {
		return "", 0, fmt.Errorf("gerrit: get change %s for patchset ref: %w", changeNumber, err)
	}

	num, err := strconv.Atoi(changeNumber)
	if err != nil {
		return "", 0, fmt.Errorf("gerrit: invalid change number %q: %w", changeNumber, err)
	}

	suffix := fmt.Sprintf("%02d", num%100)

	if change.CurrentRevision != "" && change.Revisions != nil {
		if revInfo, ok := change.Revisions[change.CurrentRevision]; ok {
			patchset = revInfo.Number
			ref = fmt.Sprintf("refs/changes/%s/%d/%d", suffix, num, patchset)
			return ref, patchset, nil
		}
	}

	// 回退：尝试从 Revisions map 取第一个
	if change.Revisions != nil {
		for _, revInfo := range change.Revisions {
			patchset = revInfo.Number
			ref = fmt.Sprintf("refs/changes/%s/%d/%d", suffix, num, patchset)
			logx.Warn("gerrit: using fallback revision for patchset ref",
				"change", changeNumber,
				"patchset", patchset,
			)
			return ref, patchset, nil
		}
	}

	return "", 0, fmt.Errorf("gerrit: change %s has no revision info", changeNumber)
}

// GetChangeProject 获取 change 所属的 project name。
func (c *Client) GetChangeProject(ctx context.Context, changeNumber string) (string, error) {
	change, _, err := c.raw.Changes.GetChange(ctx, changeNumber, nil)
	if err != nil {
		return "", fmt.Errorf("gerrit: get change %s for project: %w", changeNumber, err)
	}
	return change.Project, nil
}
