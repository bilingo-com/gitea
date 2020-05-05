package uc

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wpajqz/linker/client"
)

const (
	getBasicProfile = "/v1/user/basic/profile"
	getProfile      = "/v1/user/profile"
	searchUser      = "/v1/user/search"
	validationUser  = "/v1/user/validation"
)

type (
	BasicProfile struct {
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
	}

	Profile struct {
		Nickname     string `json:"nickname"`
		Avatar       string `json:"avatar"`
		Email        string `json:"email"`
		Mobile       string `json:"phone"`
		Gender       int    `json:"gender"`
		Introduction string `json:"introduction"`
	}

	SearchUserRequest struct {
		Keyword       string `json:"keyword"`
		Page          int    `json:"page"`
		Limit         int    `json:"limit"`
		CountryCode   string `json:"phone_code"`
		FirstLanguage int    `json:"first_language"`
		Gender        int    `json:"gender"`
		AgeGT         int    `json:"age_gt"`
		AgeLT         int    `json:"age_lt"`
	}

	SearchUserResponse struct {
		ID           int64  `json:"id"`
		Nickname     string `json:"nickname"`
		Avatar       string `json:"avatar"`
		Age          int    `json:"age"`
		Reside       string `json:"reside"`
		Introduction string `json:"introduction"`
	}

	SearchUsersResponse struct {
		List  []SearchUserResponse `json:"list"`
		Total int                  `json:"total"`
	}

	BasicProfilesResponse map[int64]BasicProfile
	ProfilesResponse      map[int64]Profile
)

func (c *Client) GetBasicProfile(id ...int64) (BasicProfilesResponse, error) {
	var resp BasicProfilesResponse

	session, err := c.Session()
	if err != nil {
		return nil, err
	}

	err = session.SyncSend(getBasicProfile, id, &client.RequestStatusCallback{
		Success: func(header, body []byte) {
			err = json.Unmarshal(body, &resp)
		},
		Error: func(code int, message string) {
			err = errors.New(message)
		},
	})

	return resp, err
}

func (c *Client) GetProfile(id ...int64) (ProfilesResponse, error) {
	var resp ProfilesResponse

	session, err := c.Session()
	if err != nil {
		return nil, err
	}

	err = session.SyncSend(getProfile, id, &client.RequestStatusCallback{
		Success: func(header, body []byte) {
			err = json.Unmarshal(body, &resp)
		},
		Error: func(code int, message string) {
			err = errors.New(message)
		},
	})

	return resp, err
}

func (c *Client) SearchUser(searchUserRequest SearchUserRequest) (SearchUsersResponse, error) {
	resp := SearchUsersResponse{
		List: make([]SearchUserResponse, 0, searchUserRequest.Limit),
	}

	session, err := c.Session()
	if err != nil {
		return resp, err
	}

	err = session.SyncSend(searchUser, searchUserRequest, &client.RequestStatusCallback{
		Success: func(header, body []byte) {
			err = json.Unmarshal(body, &resp)
		},
		Error: func(code int, message string) {
			err = errors.New(message)
		},
	})

	return resp, err
}

func (c *Client) ValidationUser(userId int64, username, password string) (bool, error) {
	if userId == 0 && username == "" {
		return false, fmt.Errorf("[ValidationUser] user_id and user_name cannot be nil at same time")
	}

	if password == "" {
		return false, fmt.Errorf("[ValidationUser] password is required")
	}

	param := map[string]interface{}{
		"user_id": userId, "user_name": username, "password": password,
	}

	session, err := c.Session()
	if err != nil {
		return false, err
	}

	var resp bool
	err = session.SyncSend(validationUser, param, &client.RequestStatusCallback{
		Success: func(header, body []byte) {
			err = json.Unmarshal(body, &resp)
		},
		Error: func(code int, message string) {
			err = errors.New(message)
		},
	})

	return resp, err
}
