package uc

import (
	"encoding/json"
	"errors"

	"github.com/wpajqz/linker/client"
)

var sendWxTemplate = "/v1/wx/template"

type TemplateRequest struct {
	ToUser      int64           `json:"touser"`                // 必须, 用户ID
	TemplateId  string          `json:"template_id"`           // 必须, 模版ID
	URL         string          `json:"url,omitempty"`         // 可选, 用户点击后跳转的URL, 该URL必须处于开发者在公众平台网站中设置的域中
	MiniProgram *MiniProgram    `json:"miniprogram,omitempty"` // 可选, 跳小程序所需数据，不需跳小程序可不用传该数据
	Data        json.RawMessage `json:"data"`                  // 必须, 模板数据, JSON 格式的 []byte, 满足特定的模板需求
}

type MiniProgram struct {
	AppId    string `json:"appid"`    // 必选; 所需跳转到的小程序appid（该小程序appid必须与发模板消息的公众号是绑定关联关系）
	PagePath string `json:"pagepath"` // 必选; 所需跳转到的小程序appid（该小程序appid必须与发模板消息的公众号是绑定关联关系）
}

type WxTemplateResponse string

func (c *Client) SendWxTemplate(tr *TemplateRequest) (*WxTemplateResponse, error) {
	var resp WxTemplateResponse

	session, err := c.Session()
	if err != nil {
		return nil, err
	}

	err = session.SyncSend(sendWxTemplate, tr, &client.RequestStatusCallback{
		Success: func(header, body []byte) {
			err = json.Unmarshal(body, &resp)
		},
		Error: func(code int, message string) {
			err = errors.New(message)
		},
	})

	return &resp, err
}
