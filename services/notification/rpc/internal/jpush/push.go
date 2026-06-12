package jpush

import (
	"encoding/json"
	"fmt"
)

// PushToDevice 推送到单个设备
func (c *Client) PushToDevice(registrationID string, notification *NotificationPayload, options *PushOptions) (*PushResult, error) {
	if registrationID == "" {
		return nil, ErrInvalidRegistrationID
	}

	return c.PushToDevices([]string{registrationID}, notification, options)
}

// PushToDevices 推送到多个设备
func (c *Client) PushToDevices(registrationIDs []string, notification *NotificationPayload, options *PushOptions) (*PushResult, error) {
	if len(registrationIDs) == 0 {
		return nil, ErrEmptyTarget
	}

	if len(registrationIDs) > 1000 {
		return nil, ErrTooManyRegistrationIDs
	}

	if notification == nil || notification.Content == "" {
		return nil, ErrEmptyContent
	}

	target := &PushTarget{
		RegistrationIDs: registrationIDs,
	}

	return c.push(target, notification, options)
}

// PushToAlias 推送到指定别名
//func (c *Client) PushToAlias(alias string, notification *NotificationPayload, options *PushOptions) (*PushResult, error) {
//	if alias == "" {
//		return nil, ErrEmptyTarget
//	}
//
//	target := &PushTarget{
//		Aliases: []string{alias},
//	}
//
//	return c.push(target, notification, options)
//}

// PushToAliases 推送到多个别名
// 限制：最多支持1000个别名，每个别名长度限制为40字节（UTF-8编码）
func (c *Client) PushToAliases(aliases []string, notification *NotificationPayload, options *PushOptions) (*PushResult, error) {
	if len(aliases) == 0 {
		return nil, ErrEmptyTarget
	}

	if len(aliases) > 1000 {
		return nil, ErrTooManyAliases
	}

	target := &PushTarget{
		Aliases: aliases,
	}

	return c.push(target, notification, options)
}

// PushToTags 推送到指定标签
func (c *Client) PushToTags(tags []string, notification *NotificationPayload, options *PushOptions) (*PushResult, error) {
	if len(tags) == 0 {
		return nil, ErrEmptyTarget
	}

	target := &PushTarget{
		Tags: tags,
	}

	return c.push(target, notification, options)
}

// PushBroadcast 广播推送（所有用户）
func (c *Client) PushBroadcast(notification *NotificationPayload, options *PushOptions) (*PushResult, error) {
	if notification == nil || notification.Content == "" {
		return nil, ErrEmptyContent
	}

	target := &PushTarget{
		All: true,
	}

	return c.push(target, notification, options)
}

// push 执行推送
func (c *Client) push(target *PushTarget, notification *NotificationPayload, options *PushOptions) (*PushResult, error) {
	// 构建推送载荷
	payload := c.buildPushPayload(target, notification, options)

	// 将payload转为JSON字符串输出，方便调试
	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
	c.logger.Infof("JPush request payload:\n%s", string(payloadJSON))

	// 发送请求
	respBody, statusCode, err := c.doRequest("POST", JPushPushPath, payload)
	if err != nil {
		return &PushResult{
			Success:    false,
			Error:      err,
			StatusCode: statusCode,
		}, err
	}

	// 解析响应
	var pushResp struct {
		MsgID  string `json:"msg_id"`
		SendNo string `json:"sendno"`
	}

	if err := json.Unmarshal(respBody, &pushResp); err != nil {
		return &PushResult{
			Success:    false,
			Error:      fmt.Errorf("failed to parse push response: %w", err),
			StatusCode: statusCode,
		}, err
	}

	c.logger.Infof("JPush success: msg_id=%s, sendno=%s", pushResp.MsgID, pushResp.SendNo)

	return &PushResult{
		Success:    true,
		MsgID:      pushResp.MsgID,
		SendNo:     pushResp.SendNo,
		StatusCode: statusCode,
	}, nil
}

// GetPushStatus 查询推送状态
func (c *Client) GetPushStatus(msgIDs []string) ([]*PushStatus, error) {
	if len(msgIDs) == 0 {
		return nil, fmt.Errorf("msg_ids cannot be empty")
	}

	// 构建查询参数
	path := fmt.Sprintf("%s?msg_ids=%s", JPushStatusPath, joinStrings(msgIDs, ","))

	// 发送请求
	respBody, statusCode, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var statusResp []struct {
		MsgID           string                 `json:"msg_id"`
		AndroidReceived int                    `json:"android_received"`
		IOSReceived     int                    `json:"ios_received"`
		IOSAPNSSent     int                    `json:"ios_apns_sent"`
		Extra           map[string]interface{} `json:"extra,omitempty"`
	}

	if err := json.Unmarshal(respBody, &statusResp); err != nil {
		return nil, fmt.Errorf("failed to parse status response (status=%d): %w", statusCode, err)
	}

	// 转换为PushStatus
	statuses := make([]*PushStatus, 0, len(statusResp))
	for _, item := range statusResp {
		statuses = append(statuses, &PushStatus{
			MsgID:           item.MsgID,
			AndroidReceived: item.AndroidReceived,
			IOSReceived:     item.IOSReceived,
			IOSAPNSSent:     item.IOSAPNSSent,
			Extra:           item.Extra,
		})
	}

	return statuses, nil
}

// joinStrings 连接字符串
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
